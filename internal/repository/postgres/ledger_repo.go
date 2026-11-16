package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"game-wallet-ledger/internal/domain"
)

type LedgerRepository struct {
	db *DB
}

func NewLedgerRepository(db *DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// Create writes an immutable balance movement entry into ledger_entries inside the given transaction.
func (r *LedgerRepository) Create(ctx context.Context, tx pgx.Tx, entry *domain.LedgerEntry) error {
	query := `
		INSERT INTO ledger_entries (id, transaction_id, wallet_id, entry_type, amount, balance_before, balance_after, currency, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}

	_, err := tx.Exec(ctx, query,
		entry.ID,
		entry.TransactionID,
		entry.WalletID,
		entry.EntryType,
		entry.Amount,
		entry.BalanceBefore,
		entry.BalanceAfter,
		entry.Currency,
		entry.Description,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create ledger entry failed: %w", err)
	}
	return nil
}

func (r *LedgerRepository) GetByWalletID(ctx context.Context, walletID string, limit, offset int) ([]*domain.LedgerEntry, error) {
	query := `
		SELECT id, transaction_id, wallet_id, entry_type, amount, balance_before, balance_after, currency, description, created_at
		FROM ledger_entries
		WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query ledger entries failed: %w", err)
	}
	defer rows.Close()

	var entries []*domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		var desc *string
		if err := rows.Scan(
			&e.ID,
			&e.TransactionID,
			&e.WalletID,
			&e.EntryType,
			&e.Amount,
			&e.BalanceBefore,
			&e.BalanceAfter,
			&e.Currency,
			&desc,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry failed: %w", err)
		}
		if desc != nil {
			e.Description = *desc
		}
		entries = append(entries, &e)
	}
	return entries, nil
}
