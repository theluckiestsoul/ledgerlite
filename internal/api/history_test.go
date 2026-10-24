package api_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/api"
)

// steppingClock advances one minute per call so ordering is deterministic.
func steppingClock() func() time.Time {
	current := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		t := current
		current = current.Add(time.Minute)
		return t
	}
}

func newHistoryAPI(t *testing.T) *testAPI {
	return newTestAPI(t, func(d *api.Deps) { d.Now = steppingClock() })
}

func (a *testAPI) seed(amounts ...int64) string {
	a.t.Helper()
	id := a.openAccount("alice", "INR", 0)
	for _, amount := range amounts {
		expectStatus(a.t, a.do("POST", "/accounts/"+id+"/deposits", "alice", map[string]any{"amount": amount}), http.StatusCreated)
	}
	return id
}

func amounts(r response) []int64 {
	out := []int64{}
	for _, tx := range r.Body["data"].([]any) {
		out = append(out, num(tx.(map[string]any)["amount"]))
	}
	return out
}

func TestHistoryEmpty(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.openAccount("alice", "INR", 0)
	res := a.do("GET", "/accounts/"+id+"/transactions", "alice", nil)
	expectStatus(t, res, http.StatusOK)
	if len(res.Body["data"].([]any)) != 0 || res.Body["nextCursor"] != nil {
		t.Fatalf("body = %v", res.Body)
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(100, 200, 300)
	got := amounts(a.do("GET", "/accounts/"+id+"/transactions", "alice", nil))
	if fmt.Sprint(got) != "[300 200 100]" {
		t.Fatalf("amounts = %v", got)
	}
}

func TestHistoryPagination(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(1, 2, 3, 4, 5)

	var seen []int64
	cursor, pages := "", 0
	for {
		path := "/accounts/" + id + "/transactions?limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		res := a.do("GET", path, "alice", nil)
		expectStatus(t, res, http.StatusOK)
		seen = append(seen, amounts(res)...)
		pages++
		next, _ := res.Body["nextCursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if pages != 3 || fmt.Sprint(seen) != "[5 4 3 2 1]" {
		t.Fatalf("pages=%d seen=%v", pages, seen)
	}
}

func TestHistoryFilterByType(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(1000)
	a.do("POST", "/accounts/"+id+"/withdrawals", "alice", map[string]any{"amount": 400})

	data := a.do("GET", "/accounts/"+id+"/transactions?type=withdrawal", "alice", nil).Body["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["type"] != "withdrawal" {
		t.Fatalf("data = %v", data)
	}
}

func TestHistoryTimeRangeIsInclusive(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(10, 20, 30, 40)
	all := a.do("GET", "/accounts/"+id+"/transactions", "alice", nil).Body["data"].([]any)
	second, third := all[2].(map[string]any), all[1].(map[string]any)

	path := fmt.Sprintf("/accounts/%s/transactions?from=%s&to=%s", id,
		url.QueryEscape(second["createdAt"].(string)), url.QueryEscape(third["createdAt"].(string)))
	got := amounts(a.do("GET", path, "alice", nil))
	if fmt.Sprint(got) != "[30 20]" {
		t.Fatalf("amounts = %v, want [30 20]", got)
	}
}

func TestHistoryDateOnlyToIncludesWholeDay(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(1, 2, 3)
	got := amounts(a.do("GET", "/accounts/"+id+"/transactions?from=2026-10-01&to=2026-10-01", "alice", nil))
	if fmt.Sprint(got) != "[3 2 1]" {
		t.Fatalf("amounts = %v", got)
	}
}

func TestHistoryPaginationWithIdenticalTimestamps(t *testing.T) {
	fixed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	a := newTestAPI(t, func(d *api.Deps) { d.Now = func() time.Time { return fixed } })
	id := a.seed(1, 2, 3, 4, 5)

	var seen []int64
	cursor := ""
	for {
		path := "/accounts/" + id + "/transactions?limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		res := a.do("GET", path, "alice", nil)
		seen = append(seen, amounts(res)...)
		next, _ := res.Body["nextCursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if fmt.Sprint(seen) != "[5 4 3 2 1]" {
		t.Fatalf("seen = %v", seen)
	}
}

func TestHistoryRequiresOwnership(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(100)
	expectStatus(t, a.do("GET", "/accounts/"+id+"/transactions", "bob", nil), http.StatusNotFound)
	expectStatus(t, a.do("GET", "/accounts/acc_missing/transactions", "alice", nil), http.StatusNotFound)
}

func TestHistoryRejectsInvalidQuery(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.seed(1)
	cases := map[string]string{
		"limit=-1":       "limit",
		"limit=0":        "limit",
		"limit=abc":      "limit",
		"limit=201":      "limit",
		"type=foo":       "type",
		"cursor=garbage": "cursor",
		"to=31-10-2026":  "to",
	}
	for qs, field := range cases {
		res := a.do("GET", "/accounts/"+id+"/transactions?"+qs, "alice", nil)
		expectStatus(t, res, http.StatusBadRequest)
		if errField(res) != field {
			t.Errorf("%s: field = %q, want %q", qs, errField(res), field)
		}
	}
	expectStatus(t, a.do("GET", "/accounts/"+id+"/transactions?limit=200", "alice", nil), http.StatusOK)
}

func TestHistoryRejectsInvalidDate(t *testing.T) {
	a := newHistoryAPI(t)
	id := a.openAccount("alice", "INR", 0)
	res := a.do("GET", "/accounts/"+id+"/transactions?from=yesterday", "alice", nil)
	expectStatus(t, res, http.StatusBadRequest)
	if errField(res) != "from" {
		t.Fatalf("field = %q", errField(res))
	}
}
