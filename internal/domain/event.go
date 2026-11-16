package domain

import (
	"encoding/json"
	"time"
)

const (
	EventPaymentSucceeded = "payment.succeeded"
	EventPaymentRefunded  = "payment.refunded"
)

// PaymentEvent represents an asynchronous message published to RabbitMQ.
type PaymentEvent struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"` // payment.succeeded or payment.refunded
	TransactionID string          `json:"transaction_id"`
	WalletID      string          `json:"wallet_id"`
	PlayerID      string          `json:"player_id"`
	Amount        int64           `json:"amount"`
	Currency      string          `json:"currency"`
	ReferenceID   string          `json:"reference_id,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
}
