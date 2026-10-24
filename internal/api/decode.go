package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

const (
	maxBodyBytes       = 16 << 10
	maxReferenceLength = 140
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return &apiError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "content-type must be application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &tooLarge):
			return &apiError{Status: http.StatusRequestEntityTooLarge, Code: "payload_too_large", Message: fmt.Sprintf("body must be at most %d bytes", maxBodyBytes)}
		case errors.As(err, &typeErr):
			return badRequest(typeErr.Field+" has the wrong type", typeErr.Field)
		default:
			return badRequest("request body is not valid JSON", "")
		}
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
