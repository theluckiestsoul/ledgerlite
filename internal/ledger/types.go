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
	ErrCurrencyMismatch  = errors.New("accounts use different currencies")
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
	Deposit     TxType = "deposit"
	Withdrawal  TxType = "withdrawal"
	TransferIn  TxType = "transfer_in"
	TransferOut TxType = "transfer_out"
)

type Transaction struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountId"`
	Type         TxType    `json:"type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balanceAfter"`
	Reference    string    `json:"reference,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`

	TransferID            string `json:"transferId,omitempty"`
	CounterpartyAccountID string `json:"counterpartyAccountId,omitempty"`
}
