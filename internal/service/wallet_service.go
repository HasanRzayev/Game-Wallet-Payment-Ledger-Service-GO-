package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"game-wallet-ledger/internal/broker/rabbitmq"
	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/internal/repository/postgres"
	"game-wallet-ledger/pkg/logger"
	"game-wallet-ledger/pkg/metrics"
)

type WalletService interface {
	CreateWallet(ctx context.Context, playerID, currency string) (*domain.Wallet, error)
	GetWallet(ctx context.Context, walletID string) (*domain.Wallet, error)
	GetWalletByPlayerAndCurrency(ctx context.Context, playerID, currency string) (*domain.Wallet, error)
	Deposit(ctx context.Context, req DepositRequest) (*TransactionResult, error)
	Purchase(ctx context.Context, req PurchaseRequest) (*TransactionResult, error)
	Refund(ctx context.Context, req RefundRequest) (*TransactionResult, error)
	GetLedgerHistory(ctx context.Context, walletID string, limit, offset int) ([]*domain.LedgerEntry, error)
}

type DepositRequest struct {
	WalletID    string          `json:"wallet_id"`
	Amount      int64           `json:"amount"`
	Currency    string          `json:"currency"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type PurchaseRequest struct {
	WalletID    string          `json:"wallet_id"`
	Amount      int64           `json:"amount"`
	Currency    string          `json:"currency"`
	ItemSKU     string          `json:"item_sku"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type RefundRequest struct {
	OriginalTransactionID string          `json:"original_transaction_id"`
	Reason                string          `json:"reason"`
	Metadata              json.RawMessage `json:"metadata,omitempty"`
}

type TransactionResult struct {
	Transaction *domain.Transaction `json:"transaction"`
	Wallet      *domain.Wallet      `json:"wallet"`
	LedgerEntry *domain.LedgerEntry `json:"ledger_entry"`
}

type walletService struct {
	db        *postgres.DB
	walletRepo *postgres.WalletRepository
	txRepo    *postgres.TransactionRepository
	ledgerRepo *postgres.LedgerRepository
	publisher rabbitmq.EventPublisher
}

func NewWalletService(
	db *postgres.DB,
	walletRepo *postgres.WalletRepository,
	txRepo *postgres.TransactionRepository,
	ledgerRepo *postgres.LedgerRepository,
	publisher rabbitmq.EventPublisher,
) WalletService {
	return &walletService{
		db:         db,
		walletRepo: walletRepo,
		txRepo:     txRepo,
		ledgerRepo: ledgerRepo,
		publisher:  publisher,
	}
}

func (s *walletService) CreateWallet(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	if playerID == "" {
		return nil, errors.New("player_id is required")
	}
	if currency == "" {
		return nil, errors.New("currency is required")
	}

	w := &domain.Wallet{
		ID:        uuid.New().String(),
		PlayerID:  playerID,
		Currency:  currency,
		Balance:   0,
		Status:    domain.WalletStatusActive,
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.walletRepo.Create(ctx, w); err != nil {
		return nil, err
	}

	return w, nil
}

func (s *walletService) GetWallet(ctx context.Context, walletID string) (*domain.Wallet, error) {
	return s.walletRepo.GetByID(ctx, walletID)
}

func (s *walletService) GetWalletByPlayerAndCurrency(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	return s.walletRepo.GetByPlayerAndCurrency(ctx, playerID, currency)
}

// Deposit increments a player's wallet balance inside a single ACID transaction,
// records an immutable CREDIT entry in ledger_entries, and logs the transaction.
func (s *walletService) Deposit(ctx context.Context, req DepositRequest) (*TransactionResult, error) {
	start := time.Now()
	defer func() {
		metrics.TransactionDuration.WithLabelValues("DEPOSIT").Observe(time.Since(start).Seconds())
	}()

	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	var res TransactionResult

	err := s.db.WithinTransaction(ctx, func(tx pgx.Tx) error {
		// Acquire pessimistic row lock
		w, err := s.walletRepo.GetByIDForUpdate(ctx, tx, req.WalletID)
		if err != nil {
			return err
		}

		if req.Currency != "" && w.Currency != req.Currency {
			return domain.ErrCurrencyMismatch
		}

		if err := w.CanCredit(req.Amount); err != nil {
			return err
		}

		oldBalance := w.Balance
		newBalance := oldBalance + req.Amount

		// Optimistic lock verification combined with pessimistic row lock
		if err := s.walletRepo.UpdateBalance(ctx, tx, w.ID, newBalance, w.Version); err != nil {
			return err
		}

		w.Balance = newBalance
		w.Version++
		w.UpdatedAt = time.Now().UTC()

		txRecord := &domain.Transaction{
			ID:          uuid.New().String(),
			WalletID:    w.ID,
			PlayerID:    w.PlayerID,
			Type:        domain.TransactionTypeDeposit,
			Amount:      req.Amount,
			Currency:    w.Currency,
			Status:      domain.TransactionStatusSuccess,
			Description: req.Description,
			Metadata:    req.Metadata,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := s.txRepo.Create(ctx, tx, txRecord); err != nil {
			return err
		}

		ledgerEntry := &domain.LedgerEntry{
			ID:            uuid.New().String(),
			TransactionID: txRecord.ID,
			WalletID:      w.ID,
			EntryType:     domain.EntryTypeCredit,
			Amount:        req.Amount,
			BalanceBefore: oldBalance,
			BalanceAfter:  newBalance,
			Currency:      w.Currency,
			Description:   fmt.Sprintf("Deposit: %s", req.Description),
			CreatedAt:     time.Now().UTC(),
		}
		if err := s.ledgerRepo.Create(ctx, tx, ledgerEntry); err != nil {
			return err
		}

		res.Wallet = w
		res.Transaction = txRecord
		res.LedgerEntry = ledgerEntry
		return nil
	})

	if err != nil {
		metrics.TransactionsTotal.WithLabelValues("DEPOSIT", "FAILED").Inc()
		return nil, err
	}

	metrics.TransactionsTotal.WithLabelValues("DEPOSIT", "SUCCESS").Inc()
	return &res, nil
}

// Purchase decrements a player's wallet balance inside a single ACID transaction,
// verifies non-negative balance constraints, writes an immutable DEBIT ledger entry,
// and publishes payment.succeeded event upon transaction commit.
func (s *walletService) Purchase(ctx context.Context, req PurchaseRequest) (*TransactionResult, error) {
	start := time.Now()
	defer func() {
		metrics.TransactionDuration.WithLabelValues("PURCHASE").Observe(time.Since(start).Seconds())
	}()

	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	var res TransactionResult

	err := s.db.WithinTransaction(ctx, func(tx pgx.Tx) error {
		// Acquire pessimistic lock on the wallet
		w, err := s.walletRepo.GetByIDForUpdate(ctx, tx, req.WalletID)
		if err != nil {
			return err
		}

		if req.Currency != "" && w.Currency != req.Currency {
			return domain.ErrCurrencyMismatch
		}

		// Application-level balance check
		if err := w.CanDebit(req.Amount); err != nil {
			return err
		}

		oldBalance := w.Balance
		newBalance := oldBalance - req.Amount

		// Database-level update with version check + CHECK (balance >= 0) constraint
		if err := s.walletRepo.UpdateBalance(ctx, tx, w.ID, newBalance, w.Version); err != nil {
			return err
		}

		w.Balance = newBalance
		w.Version++
		w.UpdatedAt = time.Now().UTC()

		txRecord := &domain.Transaction{
			ID:          uuid.New().String(),
			WalletID:    w.ID,
			PlayerID:    w.PlayerID,
			Type:        domain.TransactionTypePurchase,
			Amount:      req.Amount,
			Currency:    w.Currency,
			ReferenceID: req.ItemSKU,
			Status:      domain.TransactionStatusSuccess,
			Description: req.Description,
			Metadata:    req.Metadata,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := s.txRepo.Create(ctx, tx, txRecord); err != nil {
			return err
		}

		ledgerEntry := &domain.LedgerEntry{
			ID:            uuid.New().String(),
			TransactionID: txRecord.ID,
			WalletID:      w.ID,
			EntryType:     domain.EntryTypeDebit,
			Amount:        req.Amount,
			BalanceBefore: oldBalance,
			BalanceAfter:  newBalance,
			Currency:      w.Currency,
			Description:   fmt.Sprintf("Purchase SKU: %s", req.ItemSKU),
			CreatedAt:     time.Now().UTC(),
		}
		if err := s.ledgerRepo.Create(ctx, tx, ledgerEntry); err != nil {
			return err
		}

		res.Wallet = w
		res.Transaction = txRecord
		res.LedgerEntry = ledgerEntry
		return nil
	})

	if err != nil {
		metrics.TransactionsTotal.WithLabelValues("PURCHASE", "FAILED").Inc()
		return nil, err
	}

	metrics.TransactionsTotal.WithLabelValues("PURCHASE", "SUCCESS").Inc()

	// Publish payment.succeeded event asynchronously to RabbitMQ
	if s.publisher != nil {
		event := &domain.PaymentEvent{
			EventID:       uuid.New().String(),
			EventType:     domain.EventPaymentSucceeded,
			TransactionID: res.Transaction.ID,
			WalletID:      res.Wallet.ID,
			PlayerID:      res.Wallet.PlayerID,
			Amount:        res.Transaction.Amount,
			Currency:      res.Transaction.Currency,
			ReferenceID:   res.Transaction.ReferenceID,
			Metadata:      res.Transaction.Metadata,
			Timestamp:     time.Now().UTC(),
		}
		if pubErr := s.publisher.PublishPaymentEvent(ctx, event); pubErr != nil {
			logger.FromContext(ctx).Error("failed to publish payment.succeeded event",
				"error", pubErr.Error(),
				"transaction_id", res.Transaction.ID,
			)
			// Non-blocking for client response, event broker logging ensures traceability
		}
	}

	return &res, nil
}

// Refund reverses an earlier purchase transaction, restores the player's wallet balance,
// writes a double-entry CREDIT ledger entry, and publishes payment.refunded.
func (s *walletService) Refund(ctx context.Context, req RefundRequest) (*TransactionResult, error) {
	start := time.Now()
	defer func() {
		metrics.TransactionDuration.WithLabelValues("REFUND").Observe(time.Since(start).Seconds())
	}()

	if req.OriginalTransactionID == "" {
		return nil, errors.New("original_transaction_id is required")
	}

	var res TransactionResult

	err := s.db.WithinTransaction(ctx, func(tx pgx.Tx) error {
		// Pessimistically lock original transaction
		origTx, err := s.txRepo.GetByIDForUpdate(ctx, tx, req.OriginalTransactionID)
		if err != nil {
			return err
		}

		if origTx.Type != domain.TransactionTypePurchase {
			return domain.ErrInvalidTransactionType
		}
		if origTx.Status != domain.TransactionStatusSuccess {
			return domain.ErrOriginalTransactionFailed
		}

		// Check if a refund transaction already exists for this original transaction
		existingRefunds, err := s.txRepo.GetByReferenceID(ctx, origTx.ID)
		if err != nil {
			return err
		}
		for _, ref := range existingRefunds {
			if ref.Type == domain.TransactionTypeRefund && ref.Status == domain.TransactionStatusSuccess {
				return domain.ErrTransactionAlreadyRefunded
			}
		}

		// Pessimistically lock the wallet
		w, err := s.walletRepo.GetByIDForUpdate(ctx, tx, origTx.WalletID)
		if err != nil {
			return err
		}

		if err := w.CanCredit(origTx.Amount); err != nil {
			return err
		}

		oldBalance := w.Balance
		newBalance := oldBalance + origTx.Amount

		if err := s.walletRepo.UpdateBalance(ctx, tx, w.ID, newBalance, w.Version); err != nil {
			return err
		}

		w.Balance = newBalance
		w.Version++
		w.UpdatedAt = time.Now().UTC()

		refundTx := &domain.Transaction{
			ID:          uuid.New().String(),
			WalletID:    w.ID,
			PlayerID:    w.PlayerID,
			Type:        domain.TransactionTypeRefund,
			Amount:      origTx.Amount,
			Currency:    origTx.Currency,
			ReferenceID: origTx.ID,
			Status:      domain.TransactionStatusSuccess,
			Description: fmt.Sprintf("Refund for transaction %s. Reason: %s", origTx.ID, req.Reason),
			Metadata:    req.Metadata,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := s.txRepo.Create(ctx, tx, refundTx); err != nil {
			return err
		}

		ledgerEntry := &domain.LedgerEntry{
			ID:            uuid.New().String(),
			TransactionID: refundTx.ID,
			WalletID:      w.ID,
			EntryType:     domain.EntryTypeCredit,
			Amount:        origTx.Amount,
			BalanceBefore: oldBalance,
			BalanceAfter:  newBalance,
			Currency:      w.Currency,
			Description:   fmt.Sprintf("Refund for transaction: %s", origTx.ID),
			CreatedAt:     time.Now().UTC(),
		}
		if err := s.ledgerRepo.Create(ctx, tx, ledgerEntry); err != nil {
			return err
		}

		res.Wallet = w
		res.Transaction = refundTx
		res.LedgerEntry = ledgerEntry
		return nil
	})

	if err != nil {
		metrics.TransactionsTotal.WithLabelValues("REFUND", "FAILED").Inc()
		return nil, err
	}

	metrics.TransactionsTotal.WithLabelValues("REFUND", "SUCCESS").Inc()

	// Publish payment.refunded event to RabbitMQ
	if s.publisher != nil {
		event := &domain.PaymentEvent{
			EventID:       uuid.New().String(),
			EventType:     domain.EventPaymentRefunded,
			TransactionID: res.Transaction.ID,
			WalletID:      res.Wallet.ID,
			PlayerID:      res.Wallet.PlayerID,
			Amount:        res.Transaction.Amount,
			Currency:      res.Transaction.Currency,
			ReferenceID:   res.Transaction.ReferenceID,
			Metadata:      res.Transaction.Metadata,
			Timestamp:     time.Now().UTC(),
		}
		if pubErr := s.publisher.PublishPaymentEvent(ctx, event); pubErr != nil {
			logger.FromContext(ctx).Error("failed to publish payment.refunded event",
				"error", pubErr.Error(),
				"transaction_id", res.Transaction.ID,
			)
		}
	}

	return &res, nil
}

func (s *walletService) GetLedgerHistory(ctx context.Context, walletID string, limit, offset int) ([]*domain.LedgerEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.ledgerRepo.GetByWalletID(ctx, walletID, limit, offset)
}
