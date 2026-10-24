package ledger

import (
	"sort"
	"sync"
)

type Store struct {
	mu       sync.Mutex
	accounts map[string]*Account
	ledger   map[string][]Transaction
}

func NewStore() *Store {
	return &Store{
		accounts: make(map[string]*Account),
		ledger:   make(map[string][]Transaction),
	}
}

func (s *Store) InsertAccount(a Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.ID] = &a
}

func (s *Store) Account(id string) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return Account{}, false
	}
	return *a, true
}

func (s *Store) AccountsByOwner(ownerID string) []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Account{}
	for _, a := range s.accounts {
		if a.OwnerID == ownerID {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// Apply runs fn and records its transaction under one lock, so check-then-write is atomic.
func (s *Store) Apply(accountID string, fn func(Account) (Transaction, error)) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[accountID]
	if !ok {
		return Transaction{}, ErrNotFound
	}
	tx, err := fn(*a)
	if err != nil {
		return Transaction{}, err
	}
	a.Balance = tx.BalanceAfter
	s.ledger[accountID] = append(s.ledger[accountID], tx)
	return tx, nil
}

func (s *Store) Transactions(accountID string) []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Transaction(nil), s.ledger[accountID]...)
}
