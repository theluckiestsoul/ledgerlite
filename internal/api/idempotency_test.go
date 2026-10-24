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

func TestIdempotencyStoreCap(t *testing.T) {
	s := newIdempotencyStore(time.Hour, 1, time.Now)
	if _, err := s.begin("u", "a", "f"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.begin("u", "b", "f"); err != errKeyStoreFull {
		t.Fatalf("err = %v, want errKeyStoreFull", err)
	}
}
