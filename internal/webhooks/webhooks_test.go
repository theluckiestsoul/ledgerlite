package webhooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

func TestValidateURLBlocksNonPublicTargets(t *testing.T) {
	blocked := []string{
		"http://example.com/hooks",
		"https://127.0.0.1/hooks",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/hooks",
		"https://[::1]/hooks",
		"https://100.64.0.1/hooks",
		"https://localhost/hooks",
		"https://metadata.google.internal/",
		"https://user:pass@example.com/",
		"http://:80",
		"ftp://example.com/hooks",
	}
	for _, u := range blocked {
		if _, err := validateURL(u, false); err == nil {
			t.Errorf("%s: expected to be rejected", u)
		}
	}
	if _, err := validateURL("https://example.com/hooks", false); err != nil {
		t.Errorf("public https URL rejected: %v", err)
	}
}

func TestSafeClientBlocksPrivateAddressesAtDialTime(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	reg := NewRegistry(true)
	ep := mustRegister(t, reg, srv.URL)
	d := NewDispatcher(reg, Options{Sleep: func(time.Duration) {}})
	if d.deliver(ep, Event{ID: "evt_test", Data: ledger.Transaction{}}) {
		t.Fatal("delivery to a loopback address should fail")
	}
	if hits.Load() != 0 {
		t.Fatalf("loopback receiver was called %d times", hits.Load())
	}
}

func TestDeliverRetriesWithBackoffAndStops(t *testing.T) {
	var hits atomic.Int32
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	reg := NewRegistry(true)
	d := NewDispatcher(reg, Options{
		AllowPrivateNetworks: true,
		Sleep:                func(d time.Duration) { sleeps = append(sleeps, d) },
	})
	if d.deliver(mustRegister(t, reg, srv.URL), Event{ID: "evt_test"}) {
		t.Fatal("expected delivery to fail")
	}
	if hits.Load() != 3 {
		t.Errorf("attempts = %d, want 3", hits.Load())
	}
	if len(sleeps) != 2 || sleeps[0] != 500*time.Millisecond || sleeps[1] != time.Second {
		t.Errorf("sleeps = %v, want [500ms 1s]", sleeps)
	}
	if len(bodies) != 3 || bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("retries must resend the same event body: %v", bodies)
	}
}

func TestDeliverDoesNotRetryClientErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	reg := NewRegistry(true)
	d := NewDispatcher(reg, Options{AllowPrivateNetworks: true, Sleep: func(time.Duration) {}})
	d.deliver(mustRegister(t, reg, srv.URL), Event{ID: "evt_test"})
	if hits.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", hits.Load())
	}
}

func TestRegistryLimitAndDelete(t *testing.T) {
	r := NewRegistry(false)
	var first Endpoint
	for i := 0; i < MaxEndpointsPerOwner; i++ {
		ep, err := r.Register("u1", "https://example.com/hook")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = ep
		}
	}
	if _, err := r.Register("u1", "https://example.com/hook"); err != ErrTooManyEndpoints {
		t.Fatalf("err = %v, want ErrTooManyEndpoints", err)
	}
	if _, err := r.Register("u2", "https://example.com/hook"); err != nil {
		t.Fatalf("other owners must not be limited: %v", err)
	}
	if err := r.Delete("u2", first.ID); err != ErrEndpointNotFound {
		t.Fatalf("deleting someone else's endpoint: err = %v", err)
	}
	if err := r.Delete("u1", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register("u1", "https://example.com/hook"); err != nil {
		t.Fatalf("slot should be free after delete: %v", err)
	}
}

func mustRegister(t *testing.T, r *Registry, url string) Endpoint {
	t.Helper()
	ep, err := r.Register("u1", url)
	if err != nil {
		t.Fatal(err)
	}
	return ep
}

func TestDeletedEndpointGetsNoQueuedEvents(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	reg := NewRegistry(true)
	ep := mustRegister(t, reg, srv.URL)
	d := NewDispatcher(reg, Options{AllowPrivateNetworks: true, Sleep: func(time.Duration) {}})
	if err := reg.Delete("u1", ep.ID); err != nil {
		t.Fatal(err)
	}
	if d.deliver(ep, Event{ID: "evt_test"}) || hits.Load() != 0 {
		t.Fatalf("deleted endpoint received %d deliveries", hits.Load())
	}
}
