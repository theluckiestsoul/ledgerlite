package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const SignatureHeader = "Ledgerlite-Signature"

// Sign produces "t=<unix seconds>,v1=<hex HMAC-SHA256 of "t.payload">".
func Sign(secret string, payload []byte, timestamp int64) string {
	return fmt.Sprintf("t=%d,v1=%s", timestamp, mac(secret, timestamp, payload))
}

func Verify(secret, header string, payload []byte) bool {
	var ts int64
	var sig string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			sig = v
		}
	}
	if ts == 0 || sig == "" {
		return false
	}
	return mac(secret, ts, payload) == sig
}

func mac(secret string, timestamp int64, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(h, "%d.", timestamp)
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
