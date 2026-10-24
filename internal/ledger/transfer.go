package ledger

import "github.com/theluckiestsoul/ledgerlite/internal/money"

type TransferRequest struct {
	FromAccountID string
	ToAccountID   string
	Amount        int64
	Reference     string
}

type TransferResult struct {
	TransferID string      `json:"transferId"`
	Debit      Transaction `json:"debit"`
	Credit     Transaction `json:"credit"`
}

// Transfer moves money from an account the caller owns to any account in the same currency.
func (s *Service) Transfer(ownerID string, req TransferRequest) (TransferResult, error) {
	transferID := NewID("trf")

	debit, err := s.store.Apply(req.FromAccountID, func(a Account) (Transaction, error) {
		if a.OwnerID != ownerID {
			return Transaction{}, ErrNotFound
		}
		if a.Balance < req.Amount {
			return Transaction{}, ErrInsufficientFunds
		}
		tx := s.newTx(a, TransferOut, req.Amount, a.Balance-req.Amount, req.Reference)
		tx.TransferID, tx.CounterpartyAccountID = transferID, req.ToAccountID
		return tx, nil
	})
	if err != nil {
		return TransferResult{}, err
	}

	from, _ := s.store.Account(req.FromAccountID)
	credit, err := s.store.Apply(req.ToAccountID, func(a Account) (Transaction, error) {
		if a.Currency != from.Currency {
			return Transaction{}, ErrCurrencyMismatch
		}
		balance, err := money.Add(a.Balance, req.Amount)
		if err != nil {
			return Transaction{}, ErrBalanceLimit
		}
		tx := s.newTx(a, TransferIn, req.Amount, balance, req.Reference)
		tx.TransferID, tx.CounterpartyAccountID = transferID, from.ID
		return tx, nil
	})
	if err != nil {
		return TransferResult{}, err
	}

	return TransferResult{TransferID: transferID, Debit: debit, Credit: credit}, nil
}
