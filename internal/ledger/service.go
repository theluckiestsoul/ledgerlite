package ledger

import (
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

type Clock func() time.Time

type Service struct {
	store *Store
	now   Clock
}

func NewService(store *Store, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

func (s *Service) Open(ownerID string, c money.Currency) Account {
	a := Account{ID: NewID("acc"), OwnerID: ownerID, Currency: c, CreatedAt: s.now().UTC()}
	s.store.InsertAccount(a)
	return a
}

func (s *Service) List(ownerID string) []Account {
	return s.store.AccountsByOwner(ownerID)
}

// Get returns ErrNotFound for other users' accounts so ids can't be probed.
func (s *Service) Get(ownerID, accountID string) (Account, error) {
	a, ok := s.store.Account(accountID)
	if !ok || a.OwnerID != ownerID {
		return Account{}, ErrNotFound
	}
	return a, nil
}

func (s *Service) Deposit(ownerID, accountID string, amount int64, ref string) (Transaction, error) {
	return s.store.Apply(accountID, func(a Account) (Transaction, error) {
		if a.OwnerID != ownerID {
			return Transaction{}, ErrNotFound
		}
		balance, err := money.Add(a.Balance, amount)
		if err != nil {
			return Transaction{}, ErrBalanceLimit
		}
		return s.newTx(a, Deposit, amount, balance, ref), nil
	})
}

func (s *Service) Withdraw(ownerID, accountID string, amount int64, ref string) (Transaction, error) {
	return s.store.Apply(accountID, func(a Account) (Transaction, error) {
		if a.OwnerID != ownerID {
			return Transaction{}, ErrNotFound
		}
		if a.Balance < amount {
			return Transaction{}, ErrInsufficientFunds
		}
		return s.newTx(a, Withdrawal, amount, a.Balance-amount, ref), nil
	})
}

func (s *Service) newTx(a Account, typ TxType, amount, balanceAfter int64, ref string) Transaction {
	return Transaction{
		ID:           NewID("txn"),
		AccountID:    a.ID,
		Type:         typ,
		Amount:       amount,
		BalanceAfter: balanceAfter,
		Reference:    ref,
		CreatedAt:    s.now().UTC(),
	}
}
