package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theluckiestsoul/ledgerlite/internal/api"
)

var testKeys = map[string]string{"alice": "test_key_alice", "bob": "test_key_bob"}

type testAPI struct {
	t *testing.T
	h http.Handler
}

func newTestAPI(t *testing.T, opts ...func(*api.Deps)) *testAPI {
	t.Helper()
	deps := api.Deps{APIKeys: map[string]string{
		testKeys["alice"]: "user_alice",
		testKeys["bob"]:   "user_bob",
	}}
	for _, opt := range opts {
		opt(&deps)
	}
	return &testAPI{t: t, h: api.NewServer(deps)}
}

type response struct {
	Status int
	Header http.Header
	Body   map[string]any
}

// do sends a request as user ("" = unauthenticated); string bodies are sent as-is.
func (a *testAPI) do(method, path, user string, body any, headers ...string) response {
	a.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.Header.Set("Authorization", "Bearer "+testKeys[user])
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	res := response{Status: rec.Code, Header: rec.Header()}
	_ = json.Unmarshal(rec.Body.Bytes(), &res.Body)
	return res
}

func (a *testAPI) openAccount(user, currency string, fund int64) string {
	a.t.Helper()
	res := a.do("POST", "/accounts", user, map[string]any{"currency": currency})
	if res.Status != http.StatusCreated {
		a.t.Fatalf("open account: %d %v", res.Status, res.Body)
	}
	id := res.Body["id"].(string)
	if fund > 0 {
		dep := a.do("POST", "/accounts/"+id+"/deposits", user, map[string]any{"amount": fund})
		if dep.Status != http.StatusCreated {
			a.t.Fatalf("fund account: %d %v", dep.Status, dep.Body)
		}
	}
	return id
}

func (a *testAPI) balance(user, id string) int64 {
	a.t.Helper()
	return num(a.do("GET", "/accounts/"+id, user, nil).Body["balance"])
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func errCode(r response) string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// errField returns the first field reported in a validation error.
func errField(r response) string {
	e, _ := r.Body["error"].(map[string]any)
	d, _ := e["details"].(map[string]any)
	errs, _ := d["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	field, _ := errs[0].(map[string]any)["field"].(string)
	return field
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d (body %v)", r.Status, want, r.Body)
	}
}
