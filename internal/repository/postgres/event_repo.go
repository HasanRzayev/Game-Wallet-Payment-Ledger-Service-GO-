package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type EventRepository struct {
	db *DB
}

func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

// RecordProcessed records that an event was successfully handled. Returns true if newly inserted, false if already processed.
func (r *EventRepository) RecordProcessed(ctx context.Context, eventID, eventType, transactionID string) (bool, error) {
	query := `
		INSERT INTO processed_events (event_id, event_type, transaction_id, processed_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.Pool.Exec(ctx, query, eventID, eventType, transactionID, time.Now().UTC())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			return false, nil
		}
		return false, fmt.Errorf("record processed event failed: %w", err)
	}
	return true, nil
}

func (r *EventRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	query := `SELECT 1 FROM processed_events WHERE event_id = $1`
	var exists int
	err := r.db.Pool.QueryRow(ctx, query, eventID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check processed event failed: %w", err)
	}
	return true, nil
}
