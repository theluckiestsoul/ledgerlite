package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

const maxReferenceLength = 140

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type validator interface {
	Validate() []fieldError
}

// decode reads a JSON body into T and returns every validation error at once.
func decode[T validator](r *http.Request) (T, error) {
	var req T
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return req, &apiError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "content-type must be application/json"}
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return req, validationError([]fieldError{{Field: typeErr.Field, Message: "has the wrong type"}})
		}
		return req, badRequest("request body is not valid JSON", "")
	}
	if errs := req.Validate(); len(errs) > 0 {
		return req, validationError(errs)
	}
	return req, nil
}

func validationError(errs []fieldError) *apiError {
	return &apiError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_request",
		Message: "request validation failed",
		Details: map[string]any{"errors": errs},
	}
}

func (r openAccountRequest) Validate() []fieldError {
	r.Currency = strings.ToUpper(strings.TrimSpace(r.Currency))
	if _, ok := money.ParseCurrency(r.Currency); !ok {
		return []fieldError{{Field: "currency", Message: "must be one of INR, USD, EUR"}}
	}
	return nil
}

func (r movementRequest) Validate() []fieldError {
	var errs []fieldError
	if r.Amount <= 0 {
		errs = append(errs, fieldError{Field: "amount", Message: "must be a positive integer in minor units"})
	}
	if r.Reference != nil && len(strings.TrimSpace(*r.Reference)) > maxReferenceLength {
		errs = append(errs, fieldError{Field: "reference", Message: "must be at most 140 characters"})
	}
	return errs
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
