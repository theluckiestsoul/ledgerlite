package webhooks

import (
	"testing"
	"time"
)

var signedAt = time.Unix(1761264000, 0)

func TestSignKnownAnswer(t *testing.T) {
	const want = "t=1761264000,v1=6298009691f8572fb5b68df66787085907ca24dfba054fdaa2f374a941c88651"
	if got := Sign("whsec_test", []byte(`{"a":1}`), signedAt.Unix()); got != want {
		t.Fatalf("Sign = %s, want %s", got, want)
	}
}

func TestVerifyAt(t *testing.T) {
	payload := []byte(`{"a":1}`)
	header := Sign("whsec_test", payload, signedAt.Unix())

	cases := []struct {
		name    string
		secret  string
		header  string
		payload []byte
		now     time.Time
		want    bool
	}{
		{"valid", "whsec_test", header, payload, signedAt.Add(time.Minute), true},
		{"tampered payload", "whsec_test", header, []byte(`{"a":2}`), signedAt, false},
		{"wrong secret", "whsec_other", header, payload, signedAt, false},
		{"malformed header", "whsec_test", "garbage", payload, signedAt, false},
		{"replayed too late", "whsec_test", header, payload, signedAt.Add(10 * time.Minute), false},
		{"from the future", "whsec_test", header, payload, signedAt.Add(-10 * time.Minute), false},
	}
	for _, tc := range cases {
		if got := VerifyAt(tc.secret, tc.header, tc.payload, tc.now, DefaultTolerance); got != tc.want {
			t.Errorf("%s: VerifyAt = %v, want %v", tc.name, got, tc.want)
		}
	}
}
