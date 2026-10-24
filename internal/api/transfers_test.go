package api_test

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
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
}

func TestTransferUnknownDestination(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)
	expectStatus(t, a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": "acc_missing", "amount": 100}), http.StatusNotFound)
}

func TestTransferValidation(t *testing.T) {
	a := newTestAPI(t)
	from := a.openAccount("alice", "INR", 1000)

	missing := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "amount": 100})
	expectStatus(t, missing, http.StatusBadRequest)
	if errField(missing) != "toAccountId" {
		t.Errorf("field = %q", errField(missing))
	}

	badAmount := a.transfer("alice", newKey(), map[string]any{"fromAccountId": from, "toAccountId": from, "amount": 1.5})
	expectStatus(t, badAmount, http.StatusBadRequest)
	if errField(badAmount) != "amount" {
		t.Errorf("field = %q", errField(badAmount))
	}
}
