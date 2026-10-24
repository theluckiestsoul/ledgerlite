package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type transferRequest struct {
	FromAccountID string  `json:"fromAccountId"`
	ToAccountID   string  `json:"toAccountId"`
	Amount        int64   `json:"amount"`
	Reference     *string `json:"reference"`
}

func (s *server) createTransfer(w http.ResponseWriter, r *http.Request, userID string) error {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > maxIdempotencyKeyLength {
		return badRequest("Idempotency-Key header is required (max 255 characters)", "Idempotency-Key")
	}

	raw, err := readBody(w, r)
	if err != nil {
		return err
	}
	var req transferRequest
	if err := unmarshalJSON(raw, &req); err != nil {
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

	sum := sha256.Sum256(raw)
	prev, err := s.idempotency.begin(userID, key, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if prev != nil {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, prev.status, prev.body)
		return nil
	}

	result, err := s.accounts.Transfer(userID, ledger.TransferRequest{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Reference:     ref,
	})
	if err != nil {
		s.idempotency.release(userID, key)
		return err
	}

	s.idempotency.complete(userID, key, http.StatusCreated, result)
	writeJSON(w, http.StatusCreated, result)
	return nil
}
