package api_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestHealthIsPublic(t *testing.T) {
	a := newTestAPI(t)
	res := a.do("GET", "/health", "", nil)
	expectStatus(t, res, http.StatusOK)
	if res.Body["status"] != "ok" {
		t.Fatalf("body = %v", res.Body)
	}
}

func TestAuthentication(t *testing.T) {
	a := newTestAPI(t)
	expectStatus(t, a.do("GET", "/accounts", "", nil), http.StatusUnauthorized)

	res := a.do("GET", "/accounts", "", nil, "Authorization", "Bearer not-a-real-key")
	expectStatus(t, res, http.StatusUnauthorized)
	if errCode(res) != "unauthorized" {
		t.Fatalf("code = %q", errCode(res))
	}
}

func TestOpenAccount(t *testing.T) {
	a := newTestAPI(t)
	res := a.do("POST", "/accounts", "alice", map[string]any{"currency": "USD"})
	expectStatus(t, res, http.StatusCreated)
	if !regexp.MustCompile(`^acc_[0-9a-f]{32}$`).MatchString(res.Body["id"].(string)) {
		t.Errorf("id = %v", res.Body["id"])
	}
	if res.Body["ownerId"] != "user_alice" || res.Body["currency"] != "USD" || num(res.Body["balance"]) != 0 {
		t.Errorf("body = %v", res.Body)
	}

	lower := a.do("POST", "/accounts", "alice", map[string]any{"currency": " usd "})
	expectStatus(t, lower, http.StatusCreated)
	if lower.Body["currency"] != "USD" {
		t.Errorf("currency stored as %v, want USD", lower.Body["currency"])
	}

	bad := a.do("POST", "/accounts", "alice", map[string]any{"currency": "BTC"})
	expectStatus(t, bad, http.StatusBadRequest)
	if errField(bad) != "currency" {
		t.Errorf("field = %q", errField(bad))
	}
}

func TestListShowsOnlyOwnAccounts(t *testing.T) {
	a := newTestAPI(t)
	mine := a.openAccount("alice", "INR", 0)
	a.openAccount("bob", "INR", 0)

	data := a.do("GET", "/accounts", "alice", nil).Body["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["id"] != mine {
		t.Fatalf("data = %v", data)
	}
}

func TestOtherUsersAccountsAreHidden(t *testing.T) {
	a := newTestAPI(t)
	bobs := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.do("GET", "/accounts/"+bobs, "alice", nil), http.StatusNotFound)
	expectStatus(t, a.do("POST", "/accounts/"+bobs+"/deposits", "alice", map[string]any{"amount": 100}), http.StatusNotFound)
	expectStatus(t, a.do("POST", "/accounts/"+bobs+"/withdrawals", "alice", map[string]any{"amount": 1}), http.StatusNotFound)
}

func TestDepositAndWithdraw(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)

	dep := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 10000, "reference": "salary"})
	expectStatus(t, dep, http.StatusCreated)
	if dep.Body["type"] != "deposit" || num(dep.Body["balanceAfter"]) != 10000 || dep.Body["reference"] != "salary" {
		t.Fatalf("deposit = %v", dep.Body)
	}

	wd := a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 2500})
	expectStatus(t, wd, http.StatusCreated)
	if num(wd.Body["balanceAfter"]) != 7500 {
		t.Fatalf("withdrawal = %v", wd.Body)
	}
	if got := a.balance("alice", id); got != 7500 {
		t.Fatalf("balance = %d, want 7500", got)
	}
}

func TestWithdrawalLimits(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 500)

	res := a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 501})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "insufficient_funds" || a.balance("alice", id) != 500 {
		t.Fatalf("overdraw: %v balance=%d", res.Body, a.balance("alice", id))
	}

	exact := a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 500})
	expectStatus(t, exact, http.StatusCreated)
	if num(exact.Body["balanceAfter"]) != 0 {
		t.Fatalf("exact = %v", exact.Body)
	}
}

func TestInvalidAmounts(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)
	cases := map[string]string{
		"zero":           `{"amount":0}`,
		"negative":       `{"amount":-100}`,
		"fractional":     `{"amount":10.5}`,
		"string":         `{"amount":"100"}`,
		"above max":      `{"amount":9007199254740993}`,
		"int64 overflow": `{"amount":1e30}`,
		"missing":        `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			res := a.do("POST", "/accounts/"+id+"/deposits", "alice", body)
			expectStatus(t, res, http.StatusBadRequest)
			if errField(res) != "amount" {
				t.Fatalf("field = %q", errField(res))
			}
		})
	}
}

func TestInvalidReference(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)
	for _, ref := range []string{"", "   ", strings.Repeat("x", 141)} {
		res := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 100, "reference": ref})
		expectStatus(t, res, http.StatusBadRequest)
		if errField(res) != "reference" {
			t.Fatalf("field = %q", errField(res))
		}
	}
}

func TestValidationReportsAllErrors(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)
	res := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": -1, "reference": strings.Repeat("x", 141)})
	expectStatus(t, res, http.StatusBadRequest)
	details := res.Body["error"].(map[string]any)["details"].(map[string]any)
	if errs := details["errors"].([]any); len(errs) != 2 {
		t.Fatalf("errors = %v", errs)
	}
	if details["field"] != "amount" {
		t.Errorf("details.field = %v, want amount (kept for existing clients)", details["field"])
	}
}

func TestTypeErrorStillReportsOtherFields(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)
	res := a.do("POST", "/accounts/"+id+"/deposits", "alice", `{"amount":"ten","reference":""}`)
	expectStatus(t, res, http.StatusBadRequest)
	var fields []string
	for _, e := range res.Body["error"].(map[string]any)["details"].(map[string]any)["errors"].([]any) {
		fields = append(fields, e.(map[string]any)["field"].(string))
	}
	if strings.Join(fields, ",") != "amount,reference" {
		t.Fatalf("fields = %v, want [amount reference]", fields)
	}

	two := a.do("POST", "/accounts/"+id+"/deposits", "alice", `{"amount":"ten","reference":5}`)
	expectStatus(t, two, http.StatusBadRequest)
	errs := two.Body["error"].(map[string]any)["details"].(map[string]any)["errors"].([]any)
	if len(errs) != 2 || !strings.Contains(errs[1].(map[string]any)["message"].(string), "must be a string") {
		t.Fatalf("errors = %v", errs)
	}

	notObject := a.do("POST", "/accounts/"+id+"/deposits", "alice", `[]`)
	expectStatus(t, notObject, http.StatusBadRequest)
	if errField(notObject) != "body" {
		t.Fatalf("field = %q, want body", errField(notObject))
	}
}

func TestReferenceLimitCountsCharacters(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 0)
	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 1, "reference": strings.Repeat("₹", 140)}), http.StatusCreated)
	expectStatus(t, a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 1, "reference": strings.Repeat("₹", 141)}), http.StatusBadRequest)
}

func TestDepositBalanceLimit(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 9007199254740991)
	res := a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": 1})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "balance_limit_exceeded" {
		t.Fatalf("code = %q", errCode(res))
	}
}

func TestRequestHandling(t *testing.T) {
	a := newTestAPI(t)
	expectStatus(t, a.do("POST", "/accounts", "alice", `{"currency":`), http.StatusBadRequest)
	expectStatus(t, a.do("POST", "/accounts", "alice", "currency=INR", "Content-Type", "application/x-www-form-urlencoded"), http.StatusUnsupportedMediaType)
	expectStatus(t, a.do("POST", "/accounts", "alice", `{"currency":"INR","pad":"`+strings.Repeat("x", 20000)+`"}`), http.StatusRequestEntityTooLarge)

	res := a.do("DELETE", "/accounts", "alice", nil)
	expectStatus(t, res, http.StatusMethodNotAllowed)
	if allow := res.Header.Get("Allow"); !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") {
		t.Errorf("Allow = %q", allow)
	}
	expectStatus(t, a.do("GET", "/nope", "alice", nil), http.StatusNotFound)
}
