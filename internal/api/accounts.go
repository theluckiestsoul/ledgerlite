package api

import (
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

type openAccountRequest struct {
	Currency string `json:"currency"`
}

type movementRequest struct {
	Amount    int64   `json:"amount"`
	Reference *string `json:"reference"`
}

func (s *server) openAccount(w http.ResponseWriter, r *http.Request, userID string) error {
	req, err := decode[openAccountRequest](r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, s.accounts.Open(userID, money.Currency(req.Currency)))
	return nil
}

func (s *server) listAccounts(w http.ResponseWriter, _ *http.Request, userID string) error {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.accounts.List(userID)})
	return nil
}

func (s *server) getAccount(w http.ResponseWriter, r *http.Request, userID string) error {
	account, err := s.accounts.Get(userID, r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, account)
	return nil
}

func (s *server) deposit(w http.ResponseWriter, r *http.Request, userID string) error {
	req, err := decode[movementRequest](r)
	if err != nil {
		return err
	}
	tx, err := s.accounts.Deposit(userID, r.PathValue("id"), req.Amount, deref(req.Reference))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, tx)
	return nil
}

func (s *server) withdraw(w http.ResponseWriter, r *http.Request, userID string) error {
	req, err := decode[movementRequest](r)
	if err != nil {
		return err
	}
	tx, err := s.accounts.Withdraw(userID, r.PathValue("id"), req.Amount, deref(req.Reference))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, tx)
	return nil
}
