package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"game-wallet-ledger/internal/domain"
)

type TransactionRepository struct {
	db *DB
}

func NewTransactionRepository(db *DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, tx pgx.Tx, t *domain.Transaction) error {
	query := `
		INSERT INTO transactions (id, wallet_id, player_id, type, amount, currency, reference_id, status, description, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}

	metadataJSON := t.Metadata
	if len(metadataJSON) == 0 {
		metadataJSON = []byte("{}")
	}

	_, err := tx.Exec(ctx, query,
		t.ID,
		t.WalletID,
		t.PlayerID,
		t.Type,
		t.Amount,
		t.Currency,
		t.ReferenceID,
		t.Status,
		t.Description,
		metadataJSON,
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create transaction failed: %w", err)
	}
	return nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, player_id, type, amount, currency, reference_id, status, description, metadata, created_at, updated_at
		FROM transactions
		WHERE id = $1
	`
	row := r.db.Pool.QueryRow(ctx, query, id)
	return r.scanTransaction(row)
}

func (r *TransactionRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, player_id, type, amount, currency, reference_id, status, description, metadata, created_at, updated_at
		FROM transactions
		WHERE id = $1
		FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, id)
	return r.scanTransaction(row)
}

func (r *TransactionRepository) GetByReferenceID(ctx context.Context, referenceID string) ([]*domain.Transaction, error) {
	query := `
		SELECT id, wallet_id, player_id, type, amount, currency, reference_id, status, description, metadata, created_at, updated_at
		FROM transactions
		WHERE reference_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, referenceID)
	if err != nil {
		return nil, fmt.Errorf("query transactions by reference failed: %w", err)
	}
	defer rows.Close()

	var results []*domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		var refID *string
		var desc *string
		var meta []byte

		if err := rows.Scan(
			&t.ID,
			&t.WalletID,
			&t.PlayerID,
			&t.Type,
			&t.Amount,
			&t.Currency,
			&refID,
			&t.Status,
			&desc,
			&meta,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transaction failed: %w", err)
		}
		if refID != nil {
			t.ReferenceID = *refID
		}
		if desc != nil {
			t.Description = *desc
		}
		t.Metadata = meta
		results = append(results, &t)
	}
	return results, nil
}

func (r *TransactionRepository) scanTransaction(row pgx.Row) (*domain.Transaction, error) {
	var t domain.Transaction
	var refID *string
	var desc *string
	var meta []byte

	err := row.Scan(
		&t.ID,
		&t.WalletID,
		&t.PlayerID,
		&t.Type,
		&t.Amount,
		&t.Currency,
		&refID,
		&t.Status,
		&desc,
		&meta,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("scan transaction failed: %w", err)
	}

	if refID != nil {
		t.ReferenceID = *refID
	}
	if desc != nil {
		t.Description = *desc
	}
	t.Metadata = meta
	return &t, nil
}
