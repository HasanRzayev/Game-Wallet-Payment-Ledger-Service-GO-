package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"game-wallet-ledger/internal/config"
	"game-wallet-ledger/pkg/logger"
)

type Client struct {
	RDB *redis.Client
}

func NewClient(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	logger.FromContext(ctx).Info("connected to redis successfully", "addr", cfg.Addr())

	return &Client{RDB: rdb}, nil
}

func (c *Client) Close() error {
	if c.RDB != nil {
		return c.RDB.Close()
	}
	return nil
}
