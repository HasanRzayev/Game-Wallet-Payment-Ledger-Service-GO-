package domain

import (
	"encoding/json"
	"time"
)

type TransactionType string

const (
	TransactionTypeDeposit  TransactionType = "DEPOSIT"
	TransactionTypePurchase TransactionType = "PURCHASE"
	TransactionTypeRefund   TransactionType = "REFUND"
)

type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "PENDING"
	TransactionStatusSuccess TransactionStatus = "SUCCESS"
	TransactionStatusFailed  TransactionStatus = "FAILED"
)

// Transaction represents an individual business financial action initiated by or for a player.
type Transaction struct {
	ID          string            `json:"id"`
	WalletID    string            `json:"wallet_id"`
	PlayerID    string            `json:"player_id"`
	Type        TransactionType   `json:"type"`
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	ReferenceID string            `json:"reference_id,omitempty"` // E.g., original transaction ID for refund or order ID
	Status      TransactionStatus `json:"status"`
	Description string            `json:"description,omitempty"`
	Metadata    json.RawMessage   `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
