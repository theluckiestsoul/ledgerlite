package api

import (
	"strings"
	"testing"
)

func TestMovementRequestValidate(t *testing.T) {
	ref := strings.Repeat("x", 141)
	cases := []struct {
		name string
		req  movementRequest
		want int
	}{
		{"valid", movementRequest{Amount: 100}, 0},
		{"zero amount", movementRequest{Amount: 0}, 1},
		{"long reference", movementRequest{Amount: 1, Reference: &ref}, 1},
		{"both invalid", movementRequest{Amount: -5, Reference: &ref}, 2},
	}
	for _, tc := range cases {
		if got := len(tc.req.Validate()); got != tc.want {
			t.Errorf("%s: %d errors, want %d", tc.name, got, tc.want)
		}
	}
}

func TestOpenAccountRequestValidate(t *testing.T) {
	for _, c := range []string{"INR", "usd", " eur "} {
		if errs := (openAccountRequest{Currency: c}).Validate(); len(errs) != 0 {
			t.Errorf("%q: unexpected errors %v", c, errs)
		}
	}
	if errs := (openAccountRequest{Currency: "BTC"}).Validate(); len(errs) != 1 {
		t.Errorf("BTC: expected an error")
	}
}
