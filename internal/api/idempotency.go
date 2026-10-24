package api

import (
	"net/http"
	"sync"
	"time"
)

const maxIdempotencyKeyLength = 255

var (
	errKeyReused = &apiError{Status: http.StatusUnprocessableEntity, Code: "idempotency_key_reused",
		Message: "Idempotency-Key was already used with a different request body"}
	errKeyInProgress = &apiError{Status: http.StatusConflict, Code: "idempotency_key_in_progress",
		Message: "a request with this Idempotency-Key is still being processed"}
	errKeyStoreFull = &apiError{Status: http.StatusServiceUnavailable, Code: "idempotency_store_full",
		Message: "too many in-flight idempotent requests, retry later"}
)

type idempotencyEntry struct {
	fingerprint string
	done        bool
	status      int
	body        any
	expiresAt   time.Time
}

// idempotencyStore scopes keys per user and reserves a key before the request runs.
type idempotencyStore struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
	entries    map[string]*idempotencyEntry
}

func newIdempotencyStore(ttl time.Duration, maxEntries int, now func() time.Time) *idempotencyStore {
	return &idempotencyStore{ttl: ttl, maxEntries: maxEntries, now: now, entries: make(map[string]*idempotencyEntry)}
}

// begin returns the stored response for a completed request, or reserves the key for a new one.
func (s *idempotencyStore) begin(userID, key, fingerprint string) (*idempotencyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for k, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, k)
		}
	}

	if e, ok := s.entries[scopedKey(userID, key)]; ok {
		switch {
		case e.fingerprint != fingerprint:
			return nil, errKeyReused
		case !e.done:
			return nil, errKeyInProgress
		default:
			return e, nil
		}
	}
	if len(s.entries) >= s.maxEntries {
		return nil, errKeyStoreFull
	}
	s.entries[scopedKey(userID, key)] = &idempotencyEntry{fingerprint: fingerprint, expiresAt: now.Add(s.ttl)}
	return nil, nil
}

func (s *idempotencyStore) complete(userID, key string, status int, body any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[scopedKey(userID, key)]; ok {
		e.done, e.status, e.body = true, status, body
	}
}

// release frees a reservation after a failed request so the client can retry with the same key.
func (s *idempotencyStore) release(userID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, scopedKey(userID, key))
}

func scopedKey(userID, key string) string {
	return userID + "\x00" + key
}
