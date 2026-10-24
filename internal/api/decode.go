package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

const (
	maxBodyBytes       = 16 << 10
	maxReferenceLength = 140
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	raw, err := readBody(w, r)
	if err != nil {
		return err
	}
	return unmarshalJSON(raw, dst)
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return nil, &apiError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "content-type must be application/json"}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, &apiError{Status: http.StatusRequestEntityTooLarge, Code: "payload_too_large", Message: fmt.Sprintf("body must be at most %d bytes", maxBodyBytes)}
	}
	return raw, err
}

// unmarshalJSON decodes the first JSON value, matching json.Decoder behaviour used elsewhere.
func unmarshalJSON(raw []byte, dst any) error {
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return badRequest(typeErr.Field+" has the wrong type", typeErr.Field)
		}
		return badRequest("request body is not valid JSON", "")
	}
	return nil
}

func requireID(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return badRequest(field+" is required", field)
	}
	return nil
}

func parseCurrency(s string) (money.Currency, error) {
	c, ok := money.ParseCurrency(s)
	if !ok {
		return "", badRequest("currency must be one of INR, USD, EUR", "currency")
	}
	return c, nil
}

func validateAmount(v int64) error {
	if !money.ValidAmount(v) {
		return badRequest("amount must be a positive integer in minor units", "amount")
	}
	return nil
}

// validateReference treats a missing reference as empty but rejects an explicit empty string.
func validateReference(ref *string) (string, error) {
	if ref == nil {
		return "", nil
	}
	if *ref == "" || len(*ref) > maxReferenceLength {
		return "", badRequest(fmt.Sprintf("reference must be 1-%d characters", maxReferenceLength), "reference")
	}
	return *ref, nil
}
