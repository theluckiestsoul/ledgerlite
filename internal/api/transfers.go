package api

import (
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type transferRequest struct {
	FromAccountID string  `json:"fromAccountId"`
	ToAccountID   string  `json:"toAccountId"`
	Amount        int64   `json:"amount"`
	Reference     *string `json:"reference"`
}

type cachedResponse struct {
	status int
	body   any
}

func (s *server) createTransfer(w http.ResponseWriter, r *http.Request, userID string) error {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		return badRequest("Idempotency-Key header is required", "Idempotency-Key")
	}
	// Retrying with the same Idempotency-Key replays the original response.
	if prev, ok := s.idempotency[key]; ok {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, prev.status, prev.body)
		return nil
	}

	var req transferRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	if err := requireID(req.FromAccountID, "fromAccountId"); err != nil {
		return err
	}
	if err := requireID(req.ToAccountID, "toAccountId"); err != nil {
		return err
	}
	if err := validateAmount(req.Amount); err != nil {
		return err
	}
	ref, err := validateReference(req.Reference)
	if err != nil {
		return err
	}

	result, err := s.accounts.Transfer(userID, ledger.TransferRequest{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Reference:     ref,
	})
	if err != nil {
		return err
	}

	s.idempotency[key] = cachedResponse{status: http.StatusCreated, body: result}
	writeJSON(w, http.StatusCreated, result)
	return nil
}
