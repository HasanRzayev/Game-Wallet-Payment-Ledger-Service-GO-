package redis

import (
	"context"
	"sync"
	"time"

	"game-wallet-ledger/internal/domain"
)

type IdempotencyStore interface {
	TryAcquire(ctx context.Context, key, requestHash string, inProgressTTL time.Duration) (bool, *IdempotencyRecord, error)
	SaveCompleted(ctx context.Context, key, requestHash string, statusCode int, responseBody []byte, ttl time.Duration) error
	Release(ctx context.Context, key string) error
}

type InMemoryIdempotencyRepository struct {
	mu      sync.Mutex
	records map[string]IdempotencyRecord
	expires map[string]time.Time
}

func NewInMemoryIdempotencyRepository() *InMemoryIdempotencyRepository {
	repo := &InMemoryIdempotencyRepository{
		records: make(map[string]IdempotencyRecord),
		expires: make(map[string]time.Time),
	}
	return repo
}

func (r *InMemoryIdempotencyRepository) TryAcquire(ctx context.Context, key, requestHash string, inProgressTTL time.Duration) (bool, *IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if exp, exists := r.expires[key]; exists && now.After(exp) {
		delete(r.records, key)
		delete(r.expires, key)
	}

	existing, exists := r.records[key]
	if exists {
		if existing.Status == IdempotencyStatusInProgress {
			return false, &existing, domain.ErrIdempotencyConflict
		}
		if existing.RequestHash != "" && requestHash != "" && existing.RequestHash != requestHash {
			return false, &existing, domain.ErrIdempotencyPayloadMismatch
		}
		return false, &existing, nil
	}

	r.records[key] = IdempotencyRecord{
		Key:         key,
		RequestHash: requestHash,
		Status:      IdempotencyStatusInProgress,
		CreatedAt:   now,
	}
	r.expires[key] = now.Add(inProgressTTL)
	return true, nil, nil
}

func (r *InMemoryIdempotencyRepository) SaveCompleted(ctx context.Context, key, requestHash string, statusCode int, responseBody []byte, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.records[key] = IdempotencyRecord{
		Key:          key,
		RequestHash:  requestHash,
		Status:       IdempotencyStatusCompleted,
		StatusCode:   statusCode,
		ResponseBody: string(responseBody),
		CreatedAt:    now,
	}
	r.expires[key] = now.Add(ttl)
	return nil
}

func (r *InMemoryIdempotencyRepository) Release(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.records, key)
	delete(r.expires, key)
	return nil
}
