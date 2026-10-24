package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/api"
	"github.com/theluckiestsoul/ledgerlite/internal/webhooks"
)

type receiver struct {
	*httptest.Server
	mu        sync.Mutex
	statuses  []int
	signature []string
	bodies    [][]byte
}

func newReceiver(t *testing.T, delay time.Duration, statuses ...int) *receiver {
	rc := &receiver{statuses: statuses}
	rc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		body, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		defer rc.mu.Unlock()
		rc.bodies = append(rc.bodies, body)
		rc.signature = append(rc.signature, r.Header.Get(webhooks.SignatureHeader))
		status := http.StatusOK
		if len(rc.statuses) > 0 {
			status, rc.statuses = rc.statuses[0], rc.statuses[1:]
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(rc.Close)
	return rc
}

func (rc *receiver) count() int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return len(rc.bodies)
}

// waitFor polls because deliveries happen in the background.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func newWebhookAPI(t *testing.T) *testAPI {
	return newTestAPI(t, func(d *api.Deps) {
		d.Webhook = webhooks.Options{AllowPrivateNetworks: true, Sleep: func(time.Duration) {}}
	})
}

func TestRegisterWebhookReturnsSecretOnce(t *testing.T) {
	a := newWebhookAPI(t)
	res := a.do("POST", "/webhooks", "bob", map[string]any{"url": "https://example.com/hooks"})
	expectStatus(t, res, http.StatusCreated)
	if !regexp.MustCompile(`^whsec_[0-9a-f]{48}$`).MatchString(res.Body["secret"].(string)) {
		t.Errorf("secret = %v", res.Body["secret"])
	}

	list := a.do("GET", "/webhooks", "bob", nil).Body["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("list = %v", list)
	}
	if _, ok := list[0].(map[string]any)["secret"]; ok {
		t.Fatal("GET /webhooks must not return secrets")
	}
}

func TestListWebhooksOnlyOwn(t *testing.T) {
	a := newWebhookAPI(t)
	expectStatus(t, a.do("POST", "/webhooks", "alice", map[string]any{"url": "https://example.com/alice"}), http.StatusCreated)
	expectStatus(t, a.do("POST", "/webhooks", "bob", map[string]any{"url": "https://example.com/bob"}), http.StatusCreated)

	data := a.do("GET", "/webhooks", "alice", nil).Body["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["url"] != "https://example.com/alice" {
		t.Fatalf("alice sees %v", data)
	}
}

func TestRegisterWebhookRejectsUnsafeURLs(t *testing.T) {
	a := newTestAPI(t)
	for _, u := range []string{"not a url", "ftp://example.com/hooks", "http://example.com/hooks",
		"https://169.254.169.254/latest/meta-data", "https://127.0.0.1:8080/", "http://:80"} {
		res := a.do("POST", "/webhooks", "alice", map[string]any{"url": u})
		expectStatus(t, res, http.StatusBadRequest)
		if errField(res) != "url" {
			t.Fatalf("%q: field = %q", u, errField(res))
		}
	}
}

func TestDepositAndWithdrawalSendSignedEvents(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, 0)
	secret := a.do("POST", "/webhooks", "alice", map[string]any{"url": rc.URL}).Body["secret"].(string)
	id := a.openAccount("alice", "INR", 0)

	dep := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 700})
	expectStatus(t, dep, http.StatusCreated)
	waitFor(t, func() bool { return rc.count() == 1 })
	expectStatus(t, a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 200}), http.StatusCreated)
	waitFor(t, func() bool { return rc.count() == 2 })

	rc.mu.Lock()
	defer rc.mu.Unlock()
	var event map[string]any
	_ = json.Unmarshal(rc.bodies[0], &event)
	if event["type"] != "transaction.created" || event["data"].(map[string]any)["id"] != dep.Body["id"] {
		t.Fatalf("event = %v", event)
	}
	for i := range rc.bodies {
		if !webhooks.Verify(secret, rc.signature[i], rc.bodies[i]) {
			t.Fatalf("delivery %d signature did not verify", i)
		}
	}
}

func TestFailedWithdrawalSendsNoEvent(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, 0)
	a.do("POST", "/webhooks", "alice", map[string]any{"url": rc.URL})
	id := a.openAccount("alice", "INR", 0)

	expectStatus(t, a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 1}), http.StatusUnprocessableEntity)
	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 5}), http.StatusCreated)
	waitFor(t, func() bool { return rc.count() >= 1 })
	time.Sleep(50 * time.Millisecond)

	rc.mu.Lock()
	defer rc.mu.Unlock()
	var event map[string]any
	_ = json.Unmarshal(rc.bodies[0], &event)
	if len(rc.bodies) != 1 || event["data"].(map[string]any)["type"] != "deposit" {
		t.Fatalf("want exactly one deposit event, got %d deliveries", len(rc.bodies))
	}
}

func TestEventsOnlyReachTheOwnersEndpoints(t *testing.T) {
	a := newWebhookAPI(t)
	aliceRC, bobRC := newReceiver(t, 0), newReceiver(t, 0)
	expectStatus(t, a.do("POST", "/webhooks", "alice", map[string]any{"url": aliceRC.URL}), http.StatusCreated)
	expectStatus(t, a.do("POST", "/webhooks", "bob", map[string]any{"url": bobRC.URL}), http.StatusCreated)
	bobs := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.do("POST", "/accounts/"+bobs+"/deposits", "bob", map[string]any{"amount": 3}), http.StatusCreated)
	waitFor(t, func() bool { return bobRC.count() == 1 })
	time.Sleep(50 * time.Millisecond)
	if aliceRC.count() != 0 {
		t.Fatalf("alice's endpoint received %d of bob's events", aliceRC.count())
	}
}

func TestDeleteWebhook(t *testing.T) {
	a := newWebhookAPI(t)
	id := a.do("POST", "/webhooks", "alice", map[string]any{"url": "https://example.com/a"}).Body["id"].(string)

	expectStatus(t, a.do("DELETE", "/webhooks/"+id, "bob", nil), http.StatusNotFound)
	expectStatus(t, a.do("DELETE", "/webhooks/"+id, "alice", nil), http.StatusNoContent)
	if data := a.do("GET", "/webhooks", "alice", nil).Body["data"].([]any); len(data) != 0 {
		t.Fatalf("endpoint still listed: %v", data)
	}
}

func TestWebhookRetriesFailedDelivery(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, 0, http.StatusInternalServerError, http.StatusOK)
	a.do("POST", "/webhooks", "bob", map[string]any{"url": rc.URL})
	id := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "bob", map[string]any{"amount": 1}), http.StatusCreated)
	waitFor(t, func() bool { return rc.count() == 2 })
}

func TestSlowReceiverDoesNotBlockDeposit(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, time.Second)
	a.do("POST", "/webhooks", "bob", map[string]any{"url": rc.URL})
	id := a.openAccount("bob", "INR", 0)

	start := time.Now()
	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "bob", map[string]any{"amount": 1}), http.StatusCreated)
	if elapsed := time.Since(start); elapsed > 800*time.Millisecond {
		t.Fatalf("deposit took %v; delivery should be asynchronous", elapsed)
	}
	waitFor(t, func() bool { return rc.count() == 1 })
}
