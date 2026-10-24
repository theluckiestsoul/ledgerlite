package api

import (
	"net/http"
)

type openAccountRequest struct {
	Currency string `json:"currency"`
}

type movementRequest struct {
	Amount    int64   `json:"amount"`
	Reference *string `json:"reference"`
}

func (s *server) openAccount(w http.ResponseWriter, r *http.Request, userID string) error {
	var req openAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	currency, err := parseCurrency(req.Currency)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, s.accounts.Open(userID, currency))
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
	amount, ref, err := decodeMovement(w, r)
	if err != nil {
		return err
	}
	tx, err := s.accounts.Deposit(userID, r.PathValue("id"), amount, ref)
	if err != nil {
		return err
	}
	s.notifier.TransactionCreated(userID, tx)
	writeJSON(w, http.StatusCreated, tx)
	return nil
}

func (s *server) withdraw(w http.ResponseWriter, r *http.Request, userID string) error {
	amount, ref, err := decodeMovement(w, r)
	if err != nil {
		return err
	}
	tx, err := s.accounts.Withdraw(userID, r.PathValue("id"), amount, ref)
	if err != nil {
		return err
	}
	s.notifier.TransactionCreated(userID, tx)
	writeJSON(w, http.StatusCreated, tx)
	return nil
}

func decodeMovement(w http.ResponseWriter, r *http.Request) (int64, string, error) {
	var req movementRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return 0, "", err
	}
	if err := validateAmount(req.Amount); err != nil {
		return 0, "", err
	}
	ref, err := validateReference(req.Reference)
	if err != nil {
		return 0, "", err
	}
	return req.Amount, ref, nil
}
