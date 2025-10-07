package models

import (
	"gorm.io/gorm"
)

// Wallet holds a user's credit balance. Balance is only ever mutated
// inside a DB transaction alongside a Transaction row (see handlers/payment_handler.go)
// so a crash mid-request can never leave balance and ledger out of sync.
type Wallet struct {
	gorm.Model

	UserID  uint  `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance int64 `gorm:"not null;default:0" json:"balance_cents"`
}

type TransactionType string

const (
	TransactionCredit TransactionType = "credit"
	TransactionDebit  TransactionType = "debit"
)

// Transaction is the append-only ledger entry for every balance change.
// IdempotencyKey lets retried requests (client timeout, load balancer retry, etc.)
// be safely replayed under peak load without double-charging or double-crediting.
type Transaction struct {
	gorm.Model

	WalletID       uint            `gorm:"not null;index" json:"wallet_id"`
	Type           TransactionType `gorm:"not null;size:10" json:"type"`
	AmountCents    int64           `gorm:"not null" json:"amount_cents"`
	IdempotencyKey string          `gorm:"uniqueIndex;not null;size:100" json:"idempotency_key"`
	Reference      string          `gorm:"size:200" json:"reference,omitempty"`
}
