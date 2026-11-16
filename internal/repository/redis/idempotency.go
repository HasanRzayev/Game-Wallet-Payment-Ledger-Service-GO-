package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"game-wallet-ledger/internal/domain"
)

type IdempotencyStatus string

const (
	IdempotencyStatusInProgress IdempotencyStatus = "IN_PROGRESS"
	IdempotencyStatusCompleted  IdempotencyStatus = "COMPLETED"
)

type IdempotencyRecord struct {
	Key          string            `json:"key"`
	RequestHash  string            `json:"request_hash"`
	Status       IdempotencyStatus `json:"status"`
	StatusCode   int               `json:"status_code"`
	ResponseBody string            `json:"response_body"`
	CreatedAt    time.Time         `json:"created_at"`
}

type IdempotencyRepository struct {
	client *Client
}

func NewIdempotencyRepository(client *Client) *IdempotencyRepository {
	return &IdempotencyRepository{client: client}
}

// TryAcquire attempts to lock an idempotency key atomically using Redis SET NX.
// Returns (acquired=true, nil, nil) if successfully locked.
// If key exists, it returns (acquired=false, existingRecord, error).
func (r *IdempotencyRepository) TryAcquire(ctx context.Context, key, requestHash string, inProgressTTL time.Duration) (bool, *IdempotencyRecord, error) {
	redisKey := "idempotency:" + key

	record := IdempotencyRecord{
		Key:         key,
		RequestHash: requestHash,
		Status:      IdempotencyStatusInProgress,
		CreatedAt:   time.Now().UTC(),
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal idempotency record: %w", err)
	}

	// Try atomic set with NX (only if key does not exist)
	setSuccess, err := r.client.RDB.SetNX(ctx, redisKey, payload, inProgressTTL).Result()
	if err != nil {
		return false, nil, fmt.Errorf("redis SetNX failed for key %s: %w", key, err)
	}

	if setSuccess {
		return true, nil, nil
	}

	// Key already exists, retrieve existing record
	data, err := r.client.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key expired between SetNX and Get, retry could be attempted by caller
			return false, nil, domain.ErrIdempotencyConflict
		}
		return false, nil, fmt.Errorf("redis Get failed for key %s: %w", key, err)
	}

	var existing IdempotencyRecord
	if err := json.Unmarshal([]byte(data), &existing); err != nil {
		return false, nil, fmt.Errorf("failed to unmarshal existing idempotency record: %w", err)
	}

	if existing.Status == IdempotencyStatusInProgress {
		return false, &existing, domain.ErrIdempotencyConflict
	}

	// If already completed, verify request hash matches to prevent payload mismatch attacks/bugs
	if existing.RequestHash != "" && requestHash != "" && existing.RequestHash != requestHash {
		return false, &existing, domain.ErrIdempotencyPayloadMismatch
	}

	return false, &existing, nil
}

// SaveCompleted marks the idempotency record as COMPLETED with the given HTTP status code and response payload.
func (r *IdempotencyRepository) SaveCompleted(ctx context.Context, key, requestHash string, statusCode int, responseBody []byte, ttl time.Duration) error {
	redisKey := "idempotency:" + key

	record := IdempotencyRecord{
		Key:          key,
		RequestHash:  requestHash,
		Status:       IdempotencyStatusCompleted,
		StatusCode:   statusCode,
		ResponseBody: string(responseBody),
		CreatedAt:    time.Now().UTC(),
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal completed idempotency record: %w", err)
	}

	if err := r.client.RDB.Set(ctx, redisKey, payload, ttl).Err(); err != nil {
		return fmt.Errorf("redis Set failed to save completed idempotency record: %w", err)
	}

	return nil
}

// Release deletes the idempotency key (e.g. if request fails before processing finishes).
func (r *IdempotencyRepository) Release(ctx context.Context, key string) error {
	redisKey := "idempotency:" + key
	return r.client.RDB.Del(ctx, redisKey).Err()
}
