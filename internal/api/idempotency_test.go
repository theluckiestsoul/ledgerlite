package api

import (
	"testing"
	"time"
)

func TestIdempotencyStoreExpiresEntries(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s := newIdempotencyStore(time.Hour, 10, func() time.Time { return now })

	if _, err := s.begin("u", "k", "f1"); err != nil {
		t.Fatal(err)
	}
	s.complete("u", "k", 201, "ok")
	if e, err := s.begin("u", "k", "f1"); err != nil || e == nil {
		t.Fatalf("expected cached entry, got %v %v", e, err)
	}

	now = now.Add(2 * time.Hour)
	if e, err := s.begin("u", "k", "f2"); err != nil || e != nil {
		t.Fatalf("expected expired key to be reusable, got %v %v", e, err)
	}
}

func TestIdempotencyStoreInProgress(t *testing.T) {
	s := newIdempotencyStore(time.Hour, 10, time.Now)
	if _, err := s.begin("u", "k", "f"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.begin("u", "k", "f"); err != errKeyInProgress {
		t.Fatalf("err = %v, want errKeyInProgress", err)
	}
	s.release("u", "k")
	if _, err := s.begin("u", "k", "f"); err != nil {
		t.Fatalf("released key should be reusable: %v", err)
	}
}

func TestIdempotencyStoreCapIsPerUser(t *testing.T) {
	s := newIdempotencyStore(time.Hour, 2, time.Now)
	for _, k := range []string{"a", "b"} {
		if _, err := s.begin("noisy", k, "f"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.begin("noisy", "c", "f"); err != errKeyLimit {
		t.Fatalf("err = %v, want errKeyLimit", err)
	}
	if _, err := s.begin("quiet", "a", "f"); err != nil {
		t.Fatalf("another user must not be affected: %v", err)
	}

	// Completed keys must keep replaying until they expire, even at the cap.
	s.complete("noisy", "a", 201, "ok")
	if _, err := s.begin("noisy", "c", "f"); err != errKeyLimit {
		t.Fatalf("err = %v, want errKeyLimit", err)
	}
	if e, err := s.begin("noisy", "a", "f"); err != nil || e == nil || e.status != 201 {
		t.Fatalf("completed key should still replay: %v %v", e, err)
	}
}

func TestIdempotencyStoreCapFreesAfterExpiry(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s := newIdempotencyStore(time.Hour, 1, func() time.Time { return now })
	if _, err := s.begin("u", "a", "f"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.begin("u", "b", "f"); err != errKeyLimit {
		t.Fatalf("err = %v, want errKeyLimit", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := s.begin("u", "b", "f"); err != nil {
		t.Fatalf("expired entry should free the slot: %v", err)
	}
}
