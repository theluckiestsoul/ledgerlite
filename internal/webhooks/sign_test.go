package webhooks

import "testing"

func TestSignVerifyRoundTrip(t *testing.T) {
	payload := []byte(`{"a":1}`)
	header := Sign("whsec_test", payload, 1761264000)
	if !Verify("whsec_test", header, payload) {
		t.Fatal("expected signature to verify")
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	header := Sign("whsec_test", []byte(`{"a":1}`), 1761264000)
	if Verify("whsec_test", header, []byte(`{"a":2}`)) {
		t.Error("tampered payload verified")
	}
	if Verify("whsec_other", header, []byte(`{"a":1}`)) {
		t.Error("wrong secret verified")
	}
	if Verify("whsec_test", "garbage", []byte(`{}`)) {
		t.Error("malformed header verified")
	}
}
