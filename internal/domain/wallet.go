package domain

import (
	"time"
)

type WalletStatus string

const (
	WalletStatusActive WalletStatus = "ACTIVE"
	WalletStatusFrozen WalletStatus = "FROZEN"
	WalletStatusClosed WalletStatus = "CLOSED"
)

// Wallet represents a player's balance container in a specific currency.
// Balances are stored as 64-bit integers (e.g. minor currency units/cents: 1000 = $10.00, or in-game gold/gems).
type Wallet struct {
	ID        string       `json:"id"`
	PlayerID  string       `json:"player_id"`
	Currency  string       `json:"currency"`
	Balance   int64        `json:"balance"`
	Status    WalletStatus `json:"status"`
	Version   int64        `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (w *Wallet) CanDebit(amount int64) error {
	if w.Status != WalletStatusActive {
		return ErrWalletFrozen
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if w.Balance < amount {
		return ErrInsufficientFunds
	}
	return nil
}

func (w *Wallet) CanCredit(amount int64) error {
	if w.Status != WalletStatusActive {
		return ErrWalletFrozen
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}
