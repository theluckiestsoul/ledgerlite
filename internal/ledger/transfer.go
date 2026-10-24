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
	if req.FromAccountID == req.ToAccountID {
		return TransferResult{}, ErrSameAccount
	}
	transferID := NewID("trf")

	debit, credit, err := s.store.ApplyPair(req.FromAccountID, req.ToAccountID, func(from, to Account) (Transaction, Transaction, error) {
		if from.OwnerID != ownerID {
			return Transaction{}, Transaction{}, ErrNotFound
		}
		if from.Currency != to.Currency {
			return Transaction{}, Transaction{}, ErrCurrencyMismatch
		}
		if from.Balance < req.Amount {
			return Transaction{}, Transaction{}, ErrInsufficientFunds
		}
		toBalance, err := money.Add(to.Balance, req.Amount)
		if err != nil {
			return Transaction{}, Transaction{}, ErrBalanceLimit
		}

		debit := s.newTx(from, TransferOut, req.Amount, from.Balance-req.Amount, req.Reference)
		debit.TransferID, debit.CounterpartyAccountID = transferID, to.ID
		credit := s.newTx(to, TransferIn, req.Amount, toBalance, req.Reference)
		credit.TransferID, credit.CounterpartyAccountID = transferID, from.ID
		return debit, credit, nil
	})
	if err != nil {
		return TransferResult{}, err
	}
	return TransferResult{TransferID: transferID, Debit: debit, Credit: credit}, nil
}
