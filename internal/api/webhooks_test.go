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

func newReceiver(t *testing.T, statuses ...int) *receiver {
	rc := &receiver{statuses: statuses}
	rc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func newWebhookAPI(t *testing.T) *testAPI {
	return newTestAPI(t, func(d *api.Deps) {
		d.Webhook = webhooks.Options{Sleep: func(time.Duration) {}}
	})
}

func TestRegisterWebhook(t *testing.T) {
	a := newWebhookAPI(t)
	res := a.do("POST", "/webhooks", "bob", map[string]any{"url": "https://example.com/hooks"})
	expectStatus(t, res, http.StatusCreated)
	if !regexp.MustCompile(`^whsec_[0-9a-f]{48}$`).MatchString(res.Body["secret"].(string)) {
		t.Errorf("secret = %v", res.Body["secret"])
	}
	if res.Body["url"] != "https://example.com/hooks" {
		t.Errorf("url = %v", res.Body["url"])
	}
}

func TestListWebhooksOnlyOwn(t *testing.T) {
	a := newWebhookAPI(t)
	a.do("POST", "/webhooks", "bob", map[string]any{"url": "https://example.com/bob"})

	res := a.do("GET", "/webhooks", "alice", nil)
	expectStatus(t, res, http.StatusOK)
	for _, ep := range res.Body["data"].([]any) {
		if ep.(map[string]any)["ownerId"] != "user_alice" {
			t.Fatalf("leaked endpoint %v", ep)
		}
	}
}

func TestRegisterWebhookRejectsInvalidURLs(t *testing.T) {
	a := newWebhookAPI(t)
	for _, u := range []string{"not a url", "ftp://example.com/hooks", "javascript:alert(1)", "https://"} {
		res := a.do("POST", "/webhooks", "alice", map[string]any{"url": u})
		expectStatus(t, res, http.StatusBadRequest)
		if errField(res) != "url" {
			t.Fatalf("%q: field = %q", u, errField(res))
		}
	}
}

func TestDepositSendsSignedEvent(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t)
	secret := a.do("POST", "/webhooks", "alice", map[string]any{"url": rc.URL}).Body["secret"].(string)
	id := a.openAccount("alice", "INR", 0)

	dep := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 700})
	expectStatus(t, dep, http.StatusCreated)

	if rc.count() != 1 {
		t.Fatalf("deliveries = %d, want 1", rc.count())
	}
	var event map[string]any
	_ = json.Unmarshal(rc.bodies[0], &event)
	if event["type"] != "transaction.created" || event["data"].(map[string]any)["id"] != dep.Body["id"] {
		t.Fatalf("event = %v", event)
	}
	if !webhooks.Verify(secret, rc.signature[0], rc.bodies[0]) {
		t.Fatal("signature did not verify")
	}
}

func TestWebhookRetriesFailedDelivery(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, http.StatusInternalServerError, http.StatusOK)
	a.do("POST", "/webhooks", "bob", map[string]any{"url": rc.URL})
	id := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "bob", map[string]any{"amount": 1}), http.StatusCreated)
	if rc.count() != 2 {
		t.Fatalf("deliveries = %d, want 2", rc.count())
	}
}

func TestDepositSucceedsWhenWebhookKeepsFailing(t *testing.T) {
	a := newWebhookAPI(t)
	rc := newReceiver(t, 500, 500, 500, 500, 500)
	a.do("POST", "/webhooks", "bob", map[string]any{"url": rc.URL})
	id := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "bob", map[string]any{"amount": 1}), http.StatusCreated)
}
