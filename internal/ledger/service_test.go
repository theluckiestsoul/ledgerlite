package ledger

import (
	"errors"
	"sync"
	"testing"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

func TestConcurrentWithdrawalsNeverOverdraw(t *testing.T) {
	svc := NewService(NewStore(), nil)
	acc := svc.Open("u1", money.INR)
	if _, err := svc.Deposit("u1", acc.ID, 100, ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Withdraw("u1", acc.ID, 10, ""); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientFunds) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	got, _ := svc.Get("u1", acc.ID)
	if succeeded != 10 || got.Balance != 0 {
		t.Fatalf("succeeded=%d balance=%d, want 10 and 0", succeeded, got.Balance)
	}
}

func TestLedgerRecordsBalanceAfter(t *testing.T) {
	store := NewStore()
	svc := NewService(store, nil)
	acc := svc.Open("u1", money.USD)
	_, _ = svc.Deposit("u1", acc.ID, 500, "a")
	_, _ = svc.Withdraw("u1", acc.ID, 200, "")

	txs := store.Transactions(acc.ID)
	if len(txs) != 2 || txs[0].BalanceAfter != 500 || txs[1].BalanceAfter != 300 {
		t.Fatalf("unexpected ledger: %+v", txs)
	}
}
