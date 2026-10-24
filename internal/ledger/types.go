package ledger

import (
	"errors"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/money"
)

var (
	ErrNotFound          = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrBalanceLimit      = errors.New("balance limit exceeded")
)

type Account struct {
	ID        string         `json:"id"`
	OwnerID   string         `json:"ownerId"`
	Currency  money.Currency `json:"currency"`
	Balance   int64          `json:"balance"`
	CreatedAt time.Time      `json:"createdAt"`
}

type TxType string

const (
	Deposit    TxType = "deposit"
	Withdrawal TxType = "withdrawal"
)

type Transaction struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountId"`
	Type         TxType    `json:"type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balanceAfter"`
	Reference    string    `json:"reference,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
