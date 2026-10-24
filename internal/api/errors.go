package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type apiError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *apiError) Error() string { return e.Message }

func badRequest(message, field string) *apiError {
	e := &apiError{Status: http.StatusBadRequest, Code: "invalid_request", Message: message}
	if field != "" {
		e.Details = map[string]any{"field": field}
	}
	return e
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
	case errors.Is(err, ledger.ErrNotFound):
		ae = &apiError{Status: http.StatusNotFound, Code: "not_found", Message: err.Error()}
	case errors.Is(err, ledger.ErrInsufficientFunds):
		ae = &apiError{Status: http.StatusUnprocessableEntity, Code: "insufficient_funds", Message: err.Error()}
	case errors.Is(err, ledger.ErrSameAccount):
		ae = &apiError{Status: http.StatusUnprocessableEntity, Code: "same_account", Message: err.Error()}
	case errors.Is(err, ledger.ErrCurrencyMismatch):
		ae = &apiError{Status: http.StatusUnprocessableEntity, Code: "currency_mismatch", Message: err.Error()}
	case errors.Is(err, ledger.ErrBalanceLimit):
		ae = &apiError{Status: http.StatusUnprocessableEntity, Code: "balance_limit_exceeded", Message: err.Error()}
	default:
		log.Printf("internal error: %v", err)
		ae = &apiError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "internal server error"}
	}
	writeJSON(w, ae.Status, map[string]any{"error": ae})
}
