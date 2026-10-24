package api

import (
	"container/list"
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
	errTooManyInFlight = &apiError{Status: http.StatusTooManyRequests, Code: "too_many_in_flight",
		Message: "too many in-flight idempotent requests, retry later"}
)

type idempotencyEntry struct {
	userID      string
	key         string
	fingerprint string
	done        bool
	status      int
	body        any
	expiresAt   time.Time
	all, mine   *list.Element
}

type userKeys struct {
	byKey map[string]*idempotencyEntry
	order *list.List
}

// idempotencyStore scopes keys per user, caps each user separately and expires entries in insertion order.
type idempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	perUser int
	now     func() time.Time
	users   map[string]*userKeys
	order   *list.List
}

func newIdempotencyStore(ttl time.Duration, perUser int, now func() time.Time) *idempotencyStore {
	return &idempotencyStore{ttl: ttl, perUser: perUser, now: now, users: make(map[string]*userKeys), order: list.New()}
}

// begin returns the stored response for a completed request, or reserves the key for a new one.
func (s *idempotencyStore) begin(userID, key, fingerprint string) (*idempotencyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()

	u := s.users[userID]
	if u == nil {
		u = &userKeys{byKey: make(map[string]*idempotencyEntry), order: list.New()}
		s.users[userID] = u
	}
	if e, ok := u.byKey[key]; ok {
		switch {
		case e.fingerprint != fingerprint:
			return nil, errKeyReused
		case !e.done:
			return nil, errKeyInProgress
		default:
			return e, nil
		}
	}
	if len(u.byKey) >= s.perUser && !s.evictOldestCompleted(u) {
		return nil, errTooManyInFlight
	}

	e := &idempotencyEntry{userID: userID, key: key, fingerprint: fingerprint, expiresAt: s.now().Add(s.ttl)}
	e.all = s.order.PushBack(e)
	e.mine = u.order.PushBack(e)
	u.byKey[key] = e
	return nil, nil
}

func (s *idempotencyStore) complete(userID, key string, status int, body any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.users[userID]; u != nil {
		if e, ok := u.byKey[key]; ok {
			e.done, e.status, e.body = true, status, body
		}
	}
}

// release frees a reservation after a failed request so the client can retry with the same key.
func (s *idempotencyStore) release(userID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.users[userID]; u != nil {
		if e, ok := u.byKey[key]; ok {
			s.remove(e)
		}
	}
}

func (s *idempotencyStore) evictExpired() {
	now := s.now()
	for front := s.order.Front(); front != nil; front = s.order.Front() {
		e := front.Value.(*idempotencyEntry)
		if now.Before(e.expiresAt) {
			return
		}
		s.remove(e)
	}
}

func (s *idempotencyStore) evictOldestCompleted(u *userKeys) bool {
	for el := u.order.Front(); el != nil; el = el.Next() {
		if e := el.Value.(*idempotencyEntry); e.done {
			s.remove(e)
			return true
		}
	}
	return false
}

func (s *idempotencyStore) remove(e *idempotencyEntry) {
	u := s.users[e.userID]
	s.order.Remove(e.all)
	u.order.Remove(e.mine)
	delete(u.byKey, e.key)
	if len(u.byKey) == 0 {
		delete(s.users, e.userID)
	}
}
