package webhooks

import (
	"net/http"
	"net/http/httptest"
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

	d := NewDispatcher(NewRegistry(false), Options{Sleep: func(time.Duration) {}})
	ep := Endpoint{ID: "whk_test", URL: srv.URL, Secret: "s"}
	if d.deliver(ep, Event{ID: "evt_test", Data: ledger.Transaction{}}) {
		t.Fatal("delivery to a loopback address should fail")
	}
	if hits.Load() != 0 {
		t.Fatalf("loopback receiver was called %d times", hits.Load())
	}
}

func TestDeliverRetriesWithBackoffAndStops(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	d := NewDispatcher(NewRegistry(true), Options{
		AllowPrivateNetworks: true,
		Sleep:                func(d time.Duration) { sleeps = append(sleeps, d) },
	})
	if d.deliver(Endpoint{ID: "whk_test", URL: srv.URL, Secret: "s"}, Event{ID: "evt_test"}) {
		t.Fatal("expected delivery to fail")
	}
	if hits.Load() != 3 {
		t.Errorf("attempts = %d, want 3", hits.Load())
	}
	if len(sleeps) != 2 || sleeps[0] != 500*time.Millisecond || sleeps[1] != time.Second {
		t.Errorf("sleeps = %v, want [500ms 1s]", sleeps)
	}
}

func TestDeliverDoesNotRetryClientErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	d := NewDispatcher(NewRegistry(true), Options{AllowPrivateNetworks: true, Sleep: func(time.Duration) {}})
	d.deliver(Endpoint{ID: "whk_test", URL: srv.URL, Secret: "s"}, Event{ID: "evt_test"})
	if hits.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", hits.Load())
	}
}
