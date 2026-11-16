package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	WalletIDKey  contextKey = "wallet_id"
	PlayerIDKey  contextKey = "player_id"
)

var defaultLogger *slog.Logger

func Init(env string) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if env == "development" {
		opts.Level = slog.LevelDebug
	}

	handler = slog.NewJSONHandler(os.Stdout, opts)
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	return defaultLogger
}

func FromContext(ctx context.Context) *slog.Logger {
	l := defaultLogger
	if l == nil {
		l = slog.Default()
	}

	if ctx == nil {
		return l
	}

	attrs := make([]any, 0)
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		attrs = append(attrs, slog.String("request_id", reqID))
	}
	if walletID, ok := ctx.Value(WalletIDKey).(string); ok && walletID != "" {
		attrs = append(attrs, slog.String("wallet_id", walletID))
	}
	if playerID, ok := ctx.Value(PlayerIDKey).(string); ok && playerID != "" {
		attrs = append(attrs, slog.String("player_id", playerID))
	}

	if len(attrs) > 0 {
		return l.With(attrs...)
	}
	return l
}

func WithContext(ctx context.Context, key contextKey, val string) context.Context {
	return context.WithValue(ctx, key, val)
}
