package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/pkg/logger"
	"game-wallet-ledger/pkg/metrics"
)

// StandaloneWalletService provides an in-memory, zero-dependency implementation of WalletService
// with pessimistic row locking and double-entry ledger semantics for quick local testing and development.
type StandaloneWalletService struct {
	mu           sync.RWMutex
	wallets      map[string]*domain.Wallet
	playerIndex  map[string]map[string]*domain.Wallet // player_id -> currency -> wallet
	rowLocks     map[string]*sync.Mutex
	transactions map[string]*domain.Transaction
	refIndex     map[string][]*domain.Transaction
	ledger       map[string][]*domain.LedgerEntry
}

func NewStandaloneWalletService() WalletService {
	return &StandaloneWalletService{
		wallets:      make(map[string]*domain.Wallet),
		playerIndex:  make(map[string]map[string]*domain.Wallet),
		rowLocks:     make(map[string]*sync.Mutex),
		transactions: make(map[string]*domain.Transaction),
		refIndex:     make(map[string][]*domain.Transaction),
		ledger:       make(map[string][]*domain.LedgerEntry),
	}
}

func (s *StandaloneWalletService) getRowLock(walletID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, exists := s.rowLocks[walletID]
	if !exists {
		lock = &sync.Mutex{}
		s.rowLocks[walletID] = lock
	}
	return lock
}

func (s *StandaloneWalletService) CreateWallet(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	if playerID == "" {
		return nil, errors.New("player_id is required")
	}
	if currency == "" {
		return nil, errors.New("currency is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if curMap, ok := s.playerIndex[playerID]; ok {
		if _, exists := curMap[currency]; exists {
			return nil, domain.ErrWalletAlreadyExists
		}
	} else {
		s.playerIndex[playerID] = make(map[string]*domain.Wallet)
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

	s.wallets[w.ID] = w
	s.playerIndex[playerID][currency] = w
	return w, nil
}

func (s *StandaloneWalletService) GetWallet(ctx context.Context, walletID string) (*domain.Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, exists := s.wallets[walletID]
	if !exists {
		return nil, domain.ErrWalletNotFound
	}

	// Return a copy to avoid mutation
	cpy := *w
	return &cpy, nil
}

func (s *StandaloneWalletService) GetWalletByPlayerAndCurrency(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if curMap, ok := s.playerIndex[playerID]; ok {
		if w, ok := curMap[currency]; ok {
			cpy := *w
			return &cpy, nil
		}
	}
	return nil, domain.ErrWalletNotFound
}

func (s *StandaloneWalletService) Deposit(ctx context.Context, req DepositRequest) (*TransactionResult, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	rowLock := s.getRowLock(req.WalletID)
	rowLock.Lock()
	defer rowLock.Unlock()

	s.mu.Lock()
	w, exists := s.wallets[req.WalletID]
	s.mu.Unlock()

	if !exists {
		return nil, domain.ErrWalletNotFound
	}

	if req.Currency != "" && w.Currency != req.Currency {
		return nil, domain.ErrCurrencyMismatch
	}

	if err := w.CanCredit(req.Amount); err != nil {
		return nil, err
	}

	oldBalance := w.Balance
	newBalance := oldBalance + req.Amount

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

	s.mu.Lock()
	s.transactions[txRecord.ID] = txRecord
	s.ledger[w.ID] = append(s.ledger[w.ID], ledgerEntry)
	s.mu.Unlock()

	metrics.TransactionsTotal.WithLabelValues("DEPOSIT", "SUCCESS").Inc()

	cpyWallet := *w
	return &TransactionResult{
		Transaction: txRecord,
		Wallet:      &cpyWallet,
		LedgerEntry: ledgerEntry,
	}, nil
}

func (s *StandaloneWalletService) Purchase(ctx context.Context, req PurchaseRequest) (*TransactionResult, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	rowLock := s.getRowLock(req.WalletID)
	rowLock.Lock()
	defer rowLock.Unlock()

	s.mu.Lock()
	w, exists := s.wallets[req.WalletID]
	s.mu.Unlock()

	if !exists {
		return nil, domain.ErrWalletNotFound
	}

	if req.Currency != "" && w.Currency != req.Currency {
		return nil, domain.ErrCurrencyMismatch
	}

	if err := w.CanDebit(req.Amount); err != nil {
		metrics.TransactionsTotal.WithLabelValues("PURCHASE", "FAILED").Inc()
		return nil, err
	}

	oldBalance := w.Balance
	newBalance := oldBalance - req.Amount
	if newBalance < 0 {
		metrics.TransactionsTotal.WithLabelValues("PURCHASE", "FAILED").Inc()
		return nil, domain.ErrInsufficientFunds
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

	s.mu.Lock()
	s.transactions[txRecord.ID] = txRecord
	s.ledger[w.ID] = append(s.ledger[w.ID], ledgerEntry)
	s.mu.Unlock()

	metrics.TransactionsTotal.WithLabelValues("PURCHASE", "SUCCESS").Inc()

	// Simulate event fulfillment
	log := logger.FromContext(ctx)
	log.Info("FULFILLMENT: Granted in-game item to player (standalone)",
		"event_type", domain.EventPaymentSucceeded,
		"player_id", w.PlayerID,
		"wallet_id", w.ID,
		"transaction_id", txRecord.ID,
		"item_sku", req.ItemSKU,
	)

	cpyWallet := *w
	return &TransactionResult{
		Transaction: txRecord,
		Wallet:      &cpyWallet,
		LedgerEntry: ledgerEntry,
	}, nil
}

func (s *StandaloneWalletService) Refund(ctx context.Context, req RefundRequest) (*TransactionResult, error) {
	if req.OriginalTransactionID == "" {
		return nil, errors.New("original_transaction_id is required")
	}

	s.mu.Lock()
	origTx, exists := s.transactions[req.OriginalTransactionID]
	if !exists {
		s.mu.Unlock()
		return nil, domain.ErrTransactionNotFound
	}
	if origTx.Type != domain.TransactionTypePurchase {
		s.mu.Unlock()
		return nil, domain.ErrInvalidTransactionType
	}
	if origTx.Status != domain.TransactionStatusSuccess {
		s.mu.Unlock()
		return nil, domain.ErrOriginalTransactionFailed
	}

	// Check if already refunded
	existingRefunds := s.refIndex[origTx.ID]
	for _, ref := range existingRefunds {
		if ref.Type == domain.TransactionTypeRefund && ref.Status == domain.TransactionStatusSuccess {
			s.mu.Unlock()
			return nil, domain.ErrTransactionAlreadyRefunded
		}
	}
	s.mu.Unlock()

	rowLock := s.getRowLock(origTx.WalletID)
	rowLock.Lock()
	defer rowLock.Unlock()

	s.mu.Lock()
	w, exists := s.wallets[origTx.WalletID]
	s.mu.Unlock()

	if !exists {
		return nil, domain.ErrWalletNotFound
	}

	oldBalance := w.Balance
	newBalance := oldBalance + origTx.Amount

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
		Description: fmt.Sprintf("Refund for transaction: %s. Reason: %s", origTx.ID, req.Reason),
		Metadata:    req.Metadata,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
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

	s.mu.Lock()
	s.transactions[refundTx.ID] = refundTx
	s.refIndex[origTx.ID] = append(s.refIndex[origTx.ID], refundTx)
	s.ledger[w.ID] = append(s.ledger[w.ID], ledgerEntry)
	s.mu.Unlock()

	metrics.TransactionsTotal.WithLabelValues("REFUND", "SUCCESS").Inc()

	log := logger.FromContext(ctx)
	log.Info("FULFILLMENT: Revoked in-game item for refunded transaction (standalone)",
		"event_type", domain.EventPaymentRefunded,
		"player_id", w.PlayerID,
		"wallet_id", w.ID,
		"transaction_id", refundTx.ID,
		"reference_id", origTx.ID,
	)

	cpyWallet := *w
	return &TransactionResult{
		Transaction: refundTx,
		Wallet:      &cpyWallet,
		LedgerEntry: ledgerEntry,
	}, nil
}

func (s *StandaloneWalletService) GetLedgerHistory(ctx context.Context, walletID string, limit, offset int) ([]*domain.LedgerEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, exists := s.ledger[walletID]
	if !exists || len(entries) == 0 {
		return []*domain.LedgerEntry{}, nil
	}

	if offset >= len(entries) {
		return []*domain.LedgerEntry{}, nil
	}

	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	// Return reversed (most recent first)
	total := len(entries)
	result := make([]*domain.LedgerEntry, 0, end-offset)
	for i := total - 1 - offset; i >= 0 && len(result) < limit; i-- {
		result = append(result, entries[i])
	}

	return result, nil
}
