package domain

import "errors"

var (
	ErrWalletNotFound             = errors.New("wallet not found")
	ErrWalletAlreadyExists        = errors.New("wallet already exists for player and currency")
	ErrInsufficientFunds          = errors.New("insufficient funds")
	ErrWalletFrozen               = errors.New("wallet is frozen")
	ErrInvalidAmount              = errors.New("amount must be greater than zero")
	ErrTransactionNotFound        = errors.New("transaction not found")
	ErrTransactionAlreadyRefunded = errors.New("transaction has already been refunded")
	ErrInvalidTransactionType     = errors.New("transaction cannot be refunded: invalid type")
	ErrOriginalTransactionFailed  = errors.New("original transaction was not successful")
	ErrCurrencyMismatch           = errors.New("currency mismatch")
	ErrIdempotencyKeyMissing      = errors.New("idempotency-key header is required")
	ErrIdempotencyConflict        = errors.New("request with this idempotency key is currently processing")
	ErrIdempotencyPayloadMismatch = errors.New("idempotency key reused with different request payload")
)
