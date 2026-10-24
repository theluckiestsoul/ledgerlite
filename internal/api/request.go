package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

const (
	maxBodyBytes       = 16 << 10
	maxReferenceLength = 140
)

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// validator is implemented on pointer receivers so Validate can normalise the request in place.
type validator[T any] interface {
	*T
	Validate() []fieldError
}

// decode reads a JSON body into T and returns every validation error at once.
func decode[T any, PT validator[T]](w http.ResponseWriter, r *http.Request) (T, error) {
	var req T
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return req, &apiError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "content-type must be application/json"}
	}
	var errs []fieldError
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &tooLarge):
			return req, &apiError{Status: http.StatusRequestEntityTooLarge, Code: "payload_too_large", Message: fmt.Sprintf("body must be at most %d bytes", maxBodyBytes)}
		case errors.As(err, &typeErr) && typeErr.Field == "":
			return req, validationError([]fieldError{{Field: "body", Message: "must be a JSON object"}})
		case errors.As(err, &typeErr):
			// encoding/json keeps decoding after a type error (reporting only the first), so other fields are still validated.
			errs = append(errs, fieldError{Field: typeErr.Field, Message: "has the wrong type"})
		default:
			return req, badRequest("request body is not valid JSON", "")
		}
	}
	for _, fe := range PT(&req).Validate() {
		if len(errs) == 0 || fe.Field != errs[0].Field {
			errs = append(errs, fe)
		}
	}
	if len(errs) > 0 {
		return req, validationError(errs)
	}
	return req, nil
}

// validationError keeps details.field (first error) for existing clients alongside the full list.
func validationError(errs []fieldError) *apiError {
	return &apiError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_request",
		Message: "request validation failed",
		Details: map[string]any{"field": errs[0].Field, "errors": errs},
	}
}

func (r *openAccountRequest) Validate() []fieldError {
	c, ok := money.ParseCurrency(strings.ToUpper(strings.TrimSpace(r.Currency)))
	if !ok {
		return []fieldError{{Field: "currency", Message: "must be one of INR, USD, EUR"}}
	}
	r.Currency = string(c)
	return nil
}

func (r *movementRequest) Validate() []fieldError {
	var errs []fieldError
	if !money.ValidAmount(r.Amount) {
		errs = append(errs, fieldError{Field: "amount", Message: "must be a positive integer in minor units"})
	}
	if r.Reference != nil {
		ref := strings.TrimSpace(*r.Reference)
		if n := utf8.RuneCountInString(ref); n == 0 || n > maxReferenceLength {
			errs = append(errs, fieldError{Field: "reference", Message: fmt.Sprintf("must be a string of 1-%d characters", maxReferenceLength)})
		}
		r.Reference = &ref
	}
	return errs
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
