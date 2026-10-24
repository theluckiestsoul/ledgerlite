package api

import (
	"strings"
	"testing"
)

func TestMovementRequestValidate(t *testing.T) {
	long, blank := strings.Repeat("x", 141), "  "
	cases := []struct {
		name string
		req  movementRequest
		want int
	}{
		{"valid", movementRequest{Amount: 100}, 0},
		{"zero amount", movementRequest{Amount: 0}, 1},
		{"above max", movementRequest{Amount: 1 << 53}, 1},
		{"long reference", movementRequest{Amount: 1, Reference: &long}, 1},
		{"blank reference", movementRequest{Amount: 1, Reference: &blank}, 1},
		{"both invalid", movementRequest{Amount: -5, Reference: &long}, 2},
	}
	for _, tc := range cases {
		if got := len(tc.req.Validate()); got != tc.want {
			t.Errorf("%s: %d errors, want %d", tc.name, got, tc.want)
		}
	}
}

func TestOpenAccountRequestNormalisesCurrency(t *testing.T) {
	for _, c := range []string{"INR", "usd", " eur "} {
		req := openAccountRequest{Currency: c}
		if errs := req.Validate(); len(errs) != 0 {
			t.Errorf("%q: unexpected errors %v", c, errs)
		}
		if req.Currency != strings.ToUpper(strings.TrimSpace(c)) {
			t.Errorf("%q normalised to %q", c, req.Currency)
		}
	}
	req := openAccountRequest{Currency: "BTC"}
	if errs := req.Validate(); len(errs) != 1 {
		t.Errorf("BTC: expected an error")
	}
}
