package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SignatureHeader  = "Ledgerlite-Signature"
	DefaultTolerance = 5 * time.Minute
)

// Sign produces "t=<unix seconds>,v1=<hex HMAC-SHA256 of "t.payload">".
func Sign(secret string, payload []byte, timestamp int64) string {
	return fmt.Sprintf("t=%d,v1=%s", timestamp, mac(secret, timestamp, payload))
}

func Verify(secret, header string, payload []byte) bool {
	return VerifyAt(secret, header, payload, time.Now(), DefaultTolerance)
}

// VerifyAt rejects signatures older or newer than tolerance to stop replays.
func VerifyAt(secret, header string, payload []byte, now time.Time, tolerance time.Duration) bool {
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
	if age := now.Sub(time.Unix(ts, 0)); age > tolerance || age < -tolerance {
		return false
	}
	return hmac.Equal([]byte(mac(secret, ts, payload)), []byte(sig))
}

func mac(secret string, timestamp int64, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(h, "%d.", timestamp)
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
