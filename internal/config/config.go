package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort      int
	Environment     string
	Postgres        PostgresConfig
	Redis           RedisConfig
	RabbitMQ        RabbitMQConfig
	ShutdownTimeout time.Duration
}

type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode)
}

type RedisConfig struct {
	Host           string
	Port           int
	Password       string
	DB             int
	IdempotencyTTL time.Duration
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type RabbitMQConfig struct {
	URL          string
	ExchangeName string
	QueueName    string
	RoutingKey   string
}

func Load() *Config {
	return &Config{
		ServerPort:      getEnvAsInt("SERVER_PORT", 8080),
		Environment:     getEnv("ENVIRONMENT", "development"),
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT_SECONDS", 10*time.Second),
		Postgres: PostgresConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnvAsInt("POSTGRES_PORT", 5432),
			User:            getEnv("POSTGRES_USER", "postgres"),
			Password:        getEnv("POSTGRES_PASSWORD", "postgres"),
			DBName:          getEnv("POSTGRES_DB", "game_wallet_ledger"),
			SSLMode:         getEnv("POSTGRES_SSLMODE", "disable"),
			MaxConns:        int32(getEnvAsInt("POSTGRES_MAX_CONNS", 25)),
			MinConns:        int32(getEnvAsInt("POSTGRES_MIN_CONNS", 5)),
			MaxConnLifetime: getEnvAsDuration("POSTGRES_MAX_CONN_LIFETIME_MINUTES", 30*time.Minute),
			MaxConnIdleTime: getEnvAsDuration("POSTGRES_MAX_CONN_IDLE_MINUTES", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:           getEnv("REDIS_HOST", "localhost"),
			Port:           getEnvAsInt("REDIS_PORT", 6379),
			Password:       getEnv("REDIS_PASSWORD", ""),
			DB:             getEnvAsInt("REDIS_DB", 0),
			IdempotencyTTL: getEnvAsDuration("REDIS_IDEMPOTENCY_TTL_MINUTES", 60*time.Minute),
		},
		RabbitMQ: RabbitMQConfig{
			URL:          getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			ExchangeName: getEnv("RABBITMQ_EXCHANGE", "gaming.payments.topic"),
			QueueName:    getEnv("RABBITMQ_QUEUE", "gaming.item_fulfillment.queue"),
			RoutingKey:   getEnv("RABBITMQ_ROUTING_KEY", "payment.*"),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if valStr, ok := os.LookupEnv(key); ok && valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if valStr, ok := os.LookupEnv(key); ok && valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil {
			return time.Duration(val) * time.Second
		}
		if d, err := time.ParseDuration(valStr); err == nil {
			return d
		}
	}
	return defaultVal
}
