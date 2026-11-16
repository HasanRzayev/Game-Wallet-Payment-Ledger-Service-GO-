package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"game-wallet-ledger/internal/domain"
)

type WalletRepository struct {
	db *DB
}

func NewWalletRepository(db *DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) Create(ctx context.Context, w *domain.Wallet) error {
	query := `
		INSERT INTO wallets (id, player_id, currency, balance, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	now := time.Now().UTC()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = now
	}
	if w.Version == 0 {
		w.Version = 1
	}

	_, err := r.db.Pool.Exec(ctx, query,
		w.ID,
		w.PlayerID,
		w.Currency,
		w.Balance,
		w.Status,
		w.Version,
		w.CreatedAt,
		w.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			return domain.ErrWalletAlreadyExists
		}
		return fmt.Errorf("create wallet failed: %w", err)
	}
	return nil
}

func (r *WalletRepository) GetByID(ctx context.Context, id string) (*domain.Wallet, error) {
	query := `
		SELECT id, player_id, currency, balance, status, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`
	row := r.db.Pool.QueryRow(ctx, query, id)
	return r.scanWallet(row)
}

func (r *WalletRepository) GetByPlayerAndCurrency(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	query := `
		SELECT id, player_id, currency, balance, status, version, created_at, updated_at
		FROM wallets
		WHERE player_id = $1 AND currency = $2
	`
	row := r.db.Pool.QueryRow(ctx, query, playerID, currency)
	return r.scanWallet(row)
}

// GetByIDForUpdate fetches the wallet record with an exclusive row-level lock (SELECT ... FOR UPDATE).
// Any concurrent transaction attempting to read/update this wallet row will be blocked until tx commits or rolls back.
func (r *WalletRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id string) (*domain.Wallet, error) {
	query := `
		SELECT id, player_id, currency, balance, status, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
		FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, id)
	return r.scanWallet(row)
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, id string, newBalance int64, expectedVersion int64) error {
	query := `
		UPDATE wallets
		SET balance = $1, version = version + 1, updated_at = $2
		WHERE id = $3 AND version = $4
	`
	tag, err := tx.Exec(ctx, query, newBalance, time.Now().UTC(), id, expectedVersion)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check constraint violation (balance >= 0)
			return domain.ErrInsufficientFunds
		}
		return fmt.Errorf("update wallet balance failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("concurrent modification or wallet not found (id: %s, version: %d)", id, expectedVersion)
	}
	return nil
}

func (r *WalletRepository) scanWallet(row pgx.Row) (*domain.Wallet, error) {
	var w domain.Wallet
	err := row.Scan(
		&w.ID,
		&w.PlayerID,
		&w.Currency,
		&w.Balance,
		&w.Status,
		&w.Version,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("scan wallet failed: %w", err)
	}
	return &w, nil
}
