.PHONY: help up down restart logs ps build test lint migrate-up migrate-down load-test clean

# Environment variables
POSTGRES_CONTAINER ?= wallet-postgres
POSTGRES_DB ?= game_wallet_ledger
POSTGRES_USER ?= postgres

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Start all services in background (PostgreSQL, Redis, RabbitMQ, and API)
	docker compose up -d --build

down: ## Stop and remove all containers, networks, and volumes
	docker compose down -v

restart: down up ## Restart all services from scratch

logs: ## Follow logs from all docker containers
	docker compose logs -f

ps: ## View running status of containers
	docker compose ps

build: ## Build the Go API binary locally
	go build -ldflags="-w -s" -o bin/game-wallet-ledger cmd/api/main.go

test: ## Run all unit tests and concurrency test suite
	go test -v ./...

migrate-up: ## Execute PostgreSQL schema migrations
	docker exec -i $(POSTGRES_CONTAINER) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) < migrations/000001_init_schema.up.sql

migrate-down: ## Rollback PostgreSQL schema migrations
	docker exec -i $(POSTGRES_CONTAINER) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) < migrations/000001_init_schema.down.sql

load-test: ## Execute concurrent purchase throughput and latency benchmark
	go run scripts/loadtest.go -url=http://localhost:8080 -concurrency=25 -total=500

clean: ## Remove compiled binaries
	rm -rf bin/
