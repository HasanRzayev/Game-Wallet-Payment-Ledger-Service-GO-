package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"game-wallet-ledger/internal/broker/rabbitmq"
	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/internal/repository/postgres"
	redisRepo "game-wallet-ledger/internal/repository/redis"
	"game-wallet-ledger/internal/service"
	"game-wallet-ledger/pkg/response"
)

type Handler struct {
	walletService service.WalletService
	db            *postgres.DB
	redisClient   *redisRepo.Client
	broker        *rabbitmq.Broker
}

func NewHandler(
	walletService service.WalletService,
	db *postgres.DB,
	redisClient *redisRepo.Client,
	broker *rabbitmq.Broker,
) *Handler {
	return &Handler{
		walletService: walletService,
		db:            db,
		redisClient:   redisClient,
		broker:        broker,
	}
}

type CreateWalletRequest struct {
	PlayerID string `json:"player_id" binding:"required"`
	Currency string `json:"currency" binding:"required"`
}

type DepositBodyRequest struct {
	Amount      int64           `json:"amount" binding:"required,gt=0"`
	Currency    string          `json:"currency"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type PurchaseBodyRequest struct {
	WalletID    string          `json:"wallet_id" binding:"required"`
	Amount      int64           `json:"amount" binding:"required,gt=0"`
	Currency    string          `json:"currency"`
	ItemSKU     string          `json:"item_sku" binding:"required"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type RefundBodyRequest struct {
	OriginalTransactionID string          `json:"original_transaction_id" binding:"required"`
	Reason                string          `json:"reason"`
	Metadata              json.RawMessage `json:"metadata,omitempty"`
}

func (h *Handler) CreateWallet(c *gin.Context) {
	var req CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	w, err := h.walletService.CreateWallet(c.Request.Context(), req.PlayerID, req.Currency)
	if err != nil {
		if errors.Is(err, domain.ErrWalletAlreadyExists) {
			response.Error(c, http.StatusConflict, "WALLET_EXISTS", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, w)
}

func (h *Handler) GetWallet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "wallet id is required")
		return
	}

	w, err := h.walletService.GetWallet(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrWalletNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, w)
}

func (h *Handler) GetWalletLedger(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	entries, err := h.walletService.GetLedgerHistory(c.Request.Context(), id, limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if entries == nil {
		entries = make([]*domain.LedgerEntry, 0)
	}

	response.JSON(c, http.StatusOK, entries)
}

func (h *Handler) Deposit(c *gin.Context) {
	id := c.Param("id")
	var req DepositBodyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.walletService.Deposit(c.Request.Context(), service.DepositRequest{
		WalletID:    id,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		Metadata:    req.Metadata,
	})
	if err != nil {
		h.handleDomainError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, result)
}

func (h *Handler) Purchase(c *gin.Context) {
	var req PurchaseBodyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.walletService.Purchase(c.Request.Context(), service.PurchaseRequest{
		WalletID:    req.WalletID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		ItemSKU:     req.ItemSKU,
		Description: req.Description,
		Metadata:    req.Metadata,
	})
	if err != nil {
		h.handleDomainError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, result)
}

func (h *Handler) Refund(c *gin.Context) {
	var req RefundBodyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.walletService.Refund(c.Request.Context(), service.RefundRequest{
		OriginalTransactionID: req.OriginalTransactionID,
		Reason:                req.Reason,
		Metadata:              req.Metadata,
	})
	if err != nil {
		h.handleDomainError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, result)
}

func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := gin.H{
		"status":    "UP",
		"timestamp": time.Now().UTC(),
		"checks": gin.H{
			"database": "UP",
			"redis":    "UP",
			"rabbitmq": "UP",
		},
	}

	isHealthy := true

	if h.db != nil && h.db.Pool != nil {
		if err := h.db.Pool.Ping(ctx); err != nil {
			status["checks"].(gin.H)["database"] = "DOWN: " + err.Error()
			isHealthy = false
		}
	}

	if h.redisClient != nil && h.redisClient.RDB != nil {
		if err := h.redisClient.RDB.Ping(ctx).Err(); err != nil {
			status["checks"].(gin.H)["redis"] = "DOWN: " + err.Error()
			isHealthy = false
		}
	}

	if h.broker != nil {
		ch := h.broker.Channel()
		if ch == nil || ch.IsClosed() {
			status["checks"].(gin.H)["rabbitmq"] = "DOWN: channel closed"
			isHealthy = false
		}
	}

	if !isHealthy {
		status["status"] = "DEGRADED"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

func (h *Handler) handleDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		response.Error(c, http.StatusNotFound, "WALLET_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		response.Error(c, http.StatusPaymentRequired, "INSUFFICIENT_FUNDS", err.Error())
	case errors.Is(err, domain.ErrWalletFrozen):
		response.Error(c, http.StatusForbidden, "WALLET_FROZEN", err.Error())
	case errors.Is(err, domain.ErrInvalidAmount):
		response.Error(c, http.StatusBadRequest, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, domain.ErrTransactionNotFound):
		response.Error(c, http.StatusNotFound, "TRANSACTION_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrTransactionAlreadyRefunded):
		response.Error(c, http.StatusConflict, "ALREADY_REFUNDED", err.Error())
	case errors.Is(err, domain.ErrInvalidTransactionType):
		response.Error(c, http.StatusBadRequest, "INVALID_TRANSACTION_TYPE", err.Error())
	case errors.Is(err, domain.ErrCurrencyMismatch):
		response.Error(c, http.StatusBadRequest, "CURRENCY_MISMATCH", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func RegisterRoutes(
	router *gin.Engine,
	h *Handler,
	idempRepo redisRepo.IdempotencyStore,
	idempTTL time.Duration,
) {
	// Global middlewares
	router.Use(RequestLogger())
	router.Use(PrometheusMetrics())
	router.Use(gin.Recovery())

	// Health and observability
	router.GET("/", h.Dashboard)
	router.GET("/health", h.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := router.Group("/api/v1")
	{
		wallets := v1.Group("/wallets")
		{
			wallets.POST("", h.CreateWallet)
			wallets.GET("/:id", h.GetWallet)
			wallets.GET("/:id/ledger", h.GetWalletLedger)

			// State-mutating endpoint with mandatory Idempotency-Key
			wallets.POST("/:id/deposit", IdempotencyMiddleware(idempRepo, idempTTL), h.Deposit)
		}

		payments := v1.Group("/payments")
		{
			// State-mutating endpoints with mandatory Idempotency-Key
			payments.POST("/purchase", IdempotencyMiddleware(idempRepo, idempTTL), h.Purchase)
			payments.POST("/refund", IdempotencyMiddleware(idempRepo, idempTTL), h.Refund)
		}
	}
}

func contextWithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}
