package api_test

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func newKey() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *testAPI) transfer(user, key string, body map[string]any) response {
	a.t.Helper()
	return a.do("POST", "/transfers", user, body, "Idempotency-Key", key)
}

func TestTransferMovesMoneyAndLinksLegs(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 10000)
	to := a.openAccount("alice", "INR", 0)

	res := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 2500, "reference": "rent"})
	expectStatus(t, res, http.StatusCreated)

	debit := res.Body["debit"].(map[string]any)
	credit := res.Body["credit"].(map[string]any)
	if !strings.HasPrefix(res.Body["transferId"].(string), "trf_") {
		t.Errorf("transferId = %v", res.Body["transferId"])
	}
	if debit["type"] != "transfer_out" || debit["counterpartyAccountId"] != to {
		t.Errorf("debit = %v", debit)
	}
	if credit["type"] != "transfer_in" || credit["counterpartyAccountId"] != from || credit["transferId"] != res.Body["transferId"] {
		t.Errorf("credit = %v", credit)
	}
	if a.balance("alice", from) != 7500 || a.balance("alice", to) != 2500 {
		t.Fatalf("balances = %d, %d", a.balance("alice", from), a.balance("alice", to))
	}
}

func TestTransferToAnotherUser(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	bobs := a.openAccount("bob", "INR", 0)

	expectStatus(t, a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": bobs, "amount": 400}), http.StatusCreated)
	if got := a.balance("bob", bobs); got != 400 {
		t.Fatalf("bob balance = %d", got)
	}
}

func TestCannotSpendFromSomeoneElsesAccount(t *testing.T) {
	a := newTestAPI(t)
	bobs := a.openAccount("bob", "INR", 1000)
	mine := a.openAccount("alice", "INR", 0)

	expectStatus(t, a.transfer("alice", newKey(), map[string]any{"fromAccountId": bobs, "toAccountId": mine, "amount": 100}), http.StatusNotFound)
	if got := a.balance("bob", bobs); got != 1000 {
		t.Fatalf("bob balance = %d", got)
	}
}

func TestTransferReplaysSameIdempotencyKey(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	to := a.openAccount("alice", "INR", 0)
	key := newKey()
	body := map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 300}

	first := a.transfer("alice", key, body)
	second := a.transfer("alice", key, body)

	expectStatus(t, first, http.StatusCreated)
	expectStatus(t, second, http.StatusCreated)
	if second.Header.Get("Idempotent-Replayed") != "true" || second.Body["transferId"] != first.Body["transferId"] {
		t.Fatalf("replay = %v %v", second.Header, second.Body)
	}
	if got := a.balance("alice", from); got != 700 {
		t.Fatalf("balance = %d, want 700", got)
	}
}

func TestTransferRequiresIdempotencyKey(t *testing.T) {
	a := newTestAPI(t)
	res := a.do("POST", "/transfers", "alice", map[string]any{"fromAccountId": "a", "toAccountId": "b", "amount": 1})
	expectStatus(t, res, http.StatusBadRequest)
	if errField(res) != "Idempotency-Key" {
		t.Fatalf("field = %q", errField(res))
	}
}

func TestTransferInsufficientFunds(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 100)
	to := a.openAccount("alice", "INR", 0)

	res := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 101})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "insufficient_funds" || a.balance("alice", from) != 100 {
		t.Fatalf("res = %v", res.Body)
	}
}

func TestTransferCurrencyMismatch(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	to := a.openAccount("alice", "USD", 0)

	res := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 100})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "currency_mismatch" {
		t.Fatalf("code = %q", errCode(res))
	}
	if a.balance("alice", from) != 1000 || a.balance("alice", to) != 0 {
		t.Fatalf("balances changed: %d, %d", a.balance("alice", from), a.balance("alice", to))
	}
}

func TestTransferUnknownDestination(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	expectStatus(t, a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": "acc_missing", "amount": 100}), http.StatusNotFound)
	if got := a.balance("alice", from); got != 1000 {
		t.Fatalf("source balance = %d, want 1000", got)
	}
}

func TestTransferToSameAccountIsRejected(t *testing.T) {
	a := newTestAPI(t)
	id := a.openAccount("alice", "INR", 1000)
	res := a.transfer("alice", newKey(), map[string]any{"fromAccountId": id, "toAccountId": id, "amount": 100})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "same_account" || a.balance("alice", id) != 1000 {
		t.Fatalf("res = %v", res.Body)
	}
}

func TestIdempotencyKeyIsScopedPerUser(t *testing.T) {
	a := newTestAPI(t)
	key := newKey()
	aliceFrom, aliceTo := a.openAccount("alice", "INR", 1000), a.openAccount("alice", "INR", 0)
	bobFrom, bobTo := a.openAccount("bob", "INR", 1000), a.openAccount("bob", "INR", 0)

	first := a.transfer("alice", key, map[string]any{"fromAccountId": aliceFrom, "toAccountId": aliceTo, "amount": 100})
	second := a.transfer("bob", key, map[string]any{"fromAccountId": bobFrom, "toAccountId": bobTo, "amount": 100})

	expectStatus(t, second, http.StatusCreated)
	if second.Header.Get("Idempotent-Replayed") != "" || second.Body["transferId"] == first.Body["transferId"] {
		t.Fatalf("bob received alice's response: %v", second.Body)
	}
	if a.balance("bob", bobTo) != 100 {
		t.Fatalf("bob's transfer did not run")
	}
}

func TestIdempotencyKeyReusedWithDifferentBody(t *testing.T) {
	a := newTestAPI(t)
	from, to := a.openAccount("alice", "INR", 1000), a.openAccount("alice", "INR", 0)
	key := newKey()

	expectStatus(t, a.transfer("alice", key, map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 100}), http.StatusCreated)
	res := a.transfer("alice", key, map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 900})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "idempotency_key_reused" || a.balance("alice", from) != 900 {
		t.Fatalf("res = %v balance = %d", res.Body, a.balance("alice", from))
	}
}

func TestFailedTransferReleasesIdempotencyKey(t *testing.T) {
	a := newTestAPI(t)
	from, to := a.openAccount("alice", "INR", 100), a.openAccount("alice", "INR", 0)
	key := newKey()
	body := map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 500}

	expectStatus(t, a.transfer("alice", key, body), http.StatusUnprocessableEntity)
	a.do("POST", "/accounts/"+from+"/deposits", "alice", map[string]any{"amount": 400})
	expectStatus(t, a.transfer("alice", key, body), http.StatusCreated)
}

func TestConcurrentRequestsWithSameKeyTransferOnce(t *testing.T) {
	a := newTestAPI(t)
	from, to := a.openAccount("alice", "INR", 10000), a.openAccount("alice", "INR", 0)
	key := newKey()
	body := map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 100}

	var wg sync.WaitGroup
	var mu sync.Mutex
	executed := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := a.transfer("alice", key, body)
			switch {
			case res.Status == http.StatusCreated && res.Header.Get("Idempotent-Replayed") == "":
				mu.Lock()
				executed++
				mu.Unlock()
			case res.Status == http.StatusCreated, res.Status == http.StatusConflict && errCode(res) == "idempotency_key_in_progress":
			default:
				t.Errorf("unexpected response %d %v", res.Status, res.Body)
			}
		}()
	}
	wg.Wait()

	if executed != 1 {
		t.Fatalf("transfer executed %d times, want exactly 1", executed)
	}

	if got := a.balance("alice", from); got != 9900 {
		t.Fatalf("balance = %d, want 9900 (exactly one transfer)", got)
	}
}

func TestIdempotencyKeyLength(t *testing.T) {
	a := newTestAPI(t)
	from, to := a.openAccount("alice", "INR", 100), a.openAccount("alice", "INR", 0)
	body := map[string]any{"fromAccountId": from, "toAccountId": to, "amount": 1}

	expectStatus(t, a.transfer("alice", strings.Repeat("k", 255), body), http.StatusCreated)
	expectStatus(t, a.transfer("alice", strings.Repeat("k", 256), body), http.StatusBadRequest)
}

func TestTransferBodyTooLarge(t *testing.T) {
	a := newTestAPI(t)
	res := a.do("POST", "/transfers", "alice", `{"amount":1,"pad":"`+strings.Repeat("x", 20000)+`"}`, "Idempotency-Key", newKey())
	expectStatus(t, res, http.StatusRequestEntityTooLarge)
}

func TestTransferBalanceLimitLeavesSourceUntouched(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	full := a.openAccount("bob", "INR", 9007199254740991)

	res := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": full, "amount": 1})
	expectStatus(t, res, http.StatusUnprocessableEntity)
	if errCode(res) != "balance_limit_exceeded" || a.balance("alice", from) != 1000 {
		t.Fatalf("res = %v, source balance = %d", res.Body, a.balance("alice", from))
	}
}

func TestTransferValidation(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)

	missing := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "amount": 100})
	expectStatus(t, missing, http.StatusBadRequest)
	if errField(missing) != "toAccountId" {
		t.Errorf("field = %q", errField(missing))
	}

	cases := map[string]map[string]any{
		"amount":       {"fromAccountId": from, "toAccountId": "acc_x", "amount": 1.5},
		"zero amount":  {"fromAccountId": from, "toAccountId": "acc_x", "amount": 0},
		"negative":     {"fromAccountId": from, "toAccountId": "acc_x", "amount": -5},
		"missing":      {"fromAccountId": from, "toAccountId": "acc_x"},
		"missing from": {"toAccountId": "acc_x", "amount": 5},
		"empty ref":    {"fromAccountId": from, "toAccountId": "acc_x", "amount": 5, "reference": ""},
	}
	want := map[string]string{"amount": "amount", "zero amount": "amount", "negative": "amount", "missing": "amount", "missing from": "fromAccountId", "empty ref": "reference"}
	for name, body := range cases {
		res := a.transfer("alice", newKey(), body)
		expectStatus(t, res, http.StatusBadRequest)
		if errField(res) != want[name] {
			t.Errorf("%s: field = %q, want %q", name, errField(res), want[name])
		}
	}
}
