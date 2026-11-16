package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/internal/service"
)

// Unit tests for domain wallet logic
func TestWallet_DomainRules(t *testing.T) {
	t.Run("CanDebit succeeds with sufficient balance", func(t *testing.T) {
		w := &domain.Wallet{
			ID:       "w-1",
			PlayerID: "player-1",
			Currency: "USD",
			Balance:  1000,
			Status:   domain.WalletStatusActive,
		}
		err := w.CanDebit(500)
		assert.NoError(t, err)
	})

	t.Run("CanDebit fails on insufficient funds", func(t *testing.T) {
		w := &domain.Wallet{
			ID:       "w-1",
			PlayerID: "player-1",
			Currency: "USD",
			Balance:  200,
			Status:   domain.WalletStatusActive,
		}
		err := w.CanDebit(500)
		assert.ErrorIs(t, err, domain.ErrInsufficientFunds)
	})

	t.Run("CanDebit fails on frozen wallet", func(t *testing.T) {
		w := &domain.Wallet{
			ID:       "w-1",
			PlayerID: "player-1",
			Currency: "USD",
			Balance:  1000,
			Status:   domain.WalletStatusFrozen,
		}
		err := w.CanDebit(100)
		assert.ErrorIs(t, err, domain.ErrWalletFrozen)
	})

	t.Run("CanDebit fails on zero or negative amount", func(t *testing.T) {
		w := &domain.Wallet{
			ID:       "w-1",
			PlayerID: "player-1",
			Currency: "USD",
			Balance:  1000,
			Status:   domain.WalletStatusActive,
		}
		assert.ErrorIs(t, w.CanDebit(0), domain.ErrInvalidAmount)
		assert.ErrorIs(t, w.CanDebit(-50), domain.ErrInvalidAmount)
	})

	t.Run("CanCredit fails on frozen wallet or non-positive amount", func(t *testing.T) {
		frozen := &domain.Wallet{Status: domain.WalletStatusFrozen}
		assert.ErrorIs(t, frozen.CanCredit(100), domain.ErrWalletFrozen)

		active := &domain.Wallet{Status: domain.WalletStatusActive}
		assert.ErrorIs(t, active.CanCredit(0), domain.ErrInvalidAmount)
		assert.ErrorIs(t, active.CanCredit(-10), domain.ErrInvalidAmount)
		assert.NoError(t, active.CanCredit(100))
	})
}

// SimulatedThreadSafeLedger mocks the PostgreSQL row-level locking (SELECT ... FOR UPDATE)
// and double-entry ledger ACID behavior for isolated high-concurrency testing.
type SimulatedThreadSafeLedger struct {
	mu           sync.Mutex
	wallets      map[string]*domain.Wallet
	rowLocks     map[string]*sync.Mutex
	transactions map[string]*domain.Transaction
	refIndex     map[string][]*domain.Transaction
	ledger       map[string][]*domain.LedgerEntry
	events       []*domain.PaymentEvent
}

func NewSimulatedThreadSafeLedger() *SimulatedThreadSafeLedger {
	return &SimulatedThreadSafeLedger{
		wallets:      make(map[string]*domain.Wallet),
		rowLocks:     make(map[string]*sync.Mutex),
		transactions: make(map[string]*domain.Transaction),
		refIndex:     make(map[string][]*domain.Transaction),
		ledger:       make(map[string][]*domain.LedgerEntry),
		events:       make([]*domain.PaymentEvent, 0),
	}
}

func (s *SimulatedThreadSafeLedger) getRowLock(walletID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, exists := s.rowLocks[walletID]
	if !exists {
		lock = &sync.Mutex{}
		s.rowLocks[walletID] = lock
	}
	return lock
}

func (s *SimulatedThreadSafeLedger) CreateWallet(w *domain.Wallet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wallets[w.ID] = w
}

func (s *SimulatedThreadSafeLedger) ExecutePurchase(ctx context.Context, req service.PurchaseRequest) (*service.TransactionResult, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	// Pessimistic Row Lock on the Wallet (simulates SELECT ... FOR UPDATE)
	rowLock := s.getRowLock(req.WalletID)
	rowLock.Lock()
	defer rowLock.Unlock()

	s.mu.Lock()
	w, exists := s.wallets[req.WalletID]
	s.mu.Unlock()

	if !exists {
		return nil, domain.ErrWalletNotFound
	}

	if err := w.CanDebit(req.Amount); err != nil {
		return nil, err
	}

	oldBalance := w.Balance
	newBalance := oldBalance - req.Amount

	// Database constraint check (CHECK (balance >= 0))
	if newBalance < 0 {
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
	s.events = append(s.events, &domain.PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     domain.EventPaymentSucceeded,
		TransactionID: txRecord.ID,
		WalletID:      w.ID,
		PlayerID:      w.PlayerID,
		Amount:        txRecord.Amount,
		Currency:      txRecord.Currency,
		Timestamp:     time.Now().UTC(),
	})
	s.mu.Unlock()

	return &service.TransactionResult{
		Transaction: txRecord,
		Wallet:      w,
		LedgerEntry: ledgerEntry,
	}, nil
}

func (s *SimulatedThreadSafeLedger) ExecuteDeposit(ctx context.Context, req service.DepositRequest) (*service.TransactionResult, error) {
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
		Description:   req.Description,
		CreatedAt:     time.Now().UTC(),
	}

	s.mu.Lock()
	s.transactions[txRecord.ID] = txRecord
	s.ledger[w.ID] = append(s.ledger[w.ID], ledgerEntry)
	s.mu.Unlock()

	return &service.TransactionResult{
		Transaction: txRecord,
		Wallet:      w,
		LedgerEntry: ledgerEntry,
	}, nil
}

// Concurrency Test: 50 parallel purchase requests against a single wallet
// Verifies zero race conditions, deterministic final balance, and zero negative balances.
func TestConcurrency_50ParallelPurchases(t *testing.T) {
	sim := NewSimulatedThreadSafeLedger()

	walletID := "wallet-concurrent-test"
	initialBalance := int64(1000) // $10.00
	purchaseAmount := int64(25)   // $0.25 per purchase

	sim.CreateWallet(&domain.Wallet{
		ID:        walletID,
		PlayerID:  "player-concurrency-1",
		Currency:  "USD",
		Balance:   initialBalance,
		Status:    domain.WalletStatusActive,
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var successCount int64
	var failureCount int64

	startBarrier := make(chan struct{})

	// Launch 50 parallel goroutines
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			// Wait for barrier release to maximize contention
			<-startBarrier

			req := service.PurchaseRequest{
				WalletID:    walletID,
				Amount:      purchaseAmount,
				Currency:    "USD",
				ItemSKU:     fmt.Sprintf("item_skin_%02d", idx),
				Description: fmt.Sprintf("Concurrent purchase attempt %d", idx),
				Metadata:    json.RawMessage(`{"tier":"epic"}`),
			}

			res, err := sim.ExecutePurchase(context.Background(), req)
			if err != nil {
				if assert.ErrorIs(t, err, domain.ErrInsufficientFunds) {
					atomic.AddInt64(&failureCount, 1)
				}
			} else {
				require.NotNil(t, res)
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	// Release all 50 goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	// Assertions:
	// With 1000 balance and 25 per purchase:
	// Exactly 40 purchases must succeed (40 * 25 = 1000)
	// Exactly 10 purchases must fail (10 * 25 = 250 insufficient)
	assert.Equal(t, int64(40), successCount, "Exactly 40 purchases should succeed")
	assert.Equal(t, int64(10), failureCount, "Exactly 10 purchases should fail with ErrInsufficientFunds")
	assert.Equal(t, int64(50), successCount+failureCount, "Total attempts must equal 50")

	// Final wallet balance MUST be exactly 0, NEVER negative
	sim.mu.Lock()
	w := sim.wallets[walletID]
	ledgerEntries := sim.ledger[walletID]
	events := sim.events
	sim.mu.Unlock()

	assert.Equal(t, int64(0), w.Balance, "Final wallet balance must be zero")
	assert.True(t, w.Balance >= 0, "Balance must never drop below 0")
	assert.Len(t, ledgerEntries, 40, "Exactly 40 immutable ledger entries must be recorded")
	assert.Len(t, events, 40, "Exactly 40 payment.succeeded events must be generated")

	// Verify ledger integrity: every entry's balance_before - amount == balance_after
	for i, entry := range ledgerEntries {
		assert.Equal(t, domain.EntryTypeDebit, entry.EntryType)
		assert.Equal(t, purchaseAmount, entry.Amount)
		assert.Equal(t, entry.BalanceBefore-entry.Amount, entry.BalanceAfter, "Ledger entry %d math must balance", i)
	}
}

func TestDeposit_Success(t *testing.T) {
	sim := NewSimulatedThreadSafeLedger()
	walletID := "wallet-deposit-test"

	sim.CreateWallet(&domain.Wallet{
		ID:        walletID,
		PlayerID:  "player-deposit-1",
		Currency:  "USD",
		Balance:   100,
		Status:    domain.WalletStatusActive,
		Version:   1,
		CreatedAt: time.Now().UTC(),
	})

	res, err := sim.ExecuteDeposit(context.Background(), service.DepositRequest{
		WalletID:    walletID,
		Amount:      500,
		Currency:    "USD",
		Description: "Top-up 500 cents",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(600), res.Wallet.Balance)
	assert.Equal(t, domain.EntryTypeCredit, res.LedgerEntry.EntryType)
	assert.Equal(t, int64(100), res.LedgerEntry.BalanceBefore)
	assert.Equal(t, int64(600), res.LedgerEntry.BalanceAfter)
}
