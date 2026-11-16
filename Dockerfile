# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /bin/game-wallet-ledger \
    ./cmd/api

# Final runtime stage
FROM alpine:3.20

WORKDIR /app

# Add unprivileged user for security
RUN adduser -D -g '' appuser && \
    apk --no-cache add ca-certificates tzdata curl

# Copy binary and migrations from builder
COPY --from=builder /bin/game-wallet-ledger /app/game-wallet-ledger
COPY --from=builder /app/migrations /app/migrations

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/game-wallet-ledger"]
