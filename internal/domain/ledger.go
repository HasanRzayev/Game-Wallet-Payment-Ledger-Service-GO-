package domain

import (
	"time"
)

type EntryType string

const (
	EntryTypeCredit EntryType = "CREDIT" // Increases balance (Deposit, Refund)
	EntryTypeDebit  EntryType = "DEBIT"  // Decreases balance (Purchase)
)

// LedgerEntry represents an immutable single or double-entry record of balance movement.
type LedgerEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	WalletID      string    `json:"wallet_id"`
	EntryType     EntryType `json:"entry_type"`
	Amount        int64     `json:"amount"`
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	Currency      string    `json:"currency"`
	Description   string    `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
