package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"game-wallet-ledger/internal/domain"
	redisRepo "game-wallet-ledger/internal/repository/redis"
	"game-wallet-ledger/pkg/logger"
	"game-wallet-ledger/pkg/metrics"
	"game-wallet-ledger/pkg/response"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// RequestLogger logs each request with structured JSON logging and extracts or generates X-Request-ID.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Header("X-Request-ID", reqID)

		ctx := logger.WithContext(c.Request.Context(), logger.RequestIDKey, reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log := logger.FromContext(c.Request.Context())
		log.Info("http request",
			"status", status,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"ip", c.ClientIP(),
			"latency_ms", latency.Milliseconds(),
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// PrometheusMetrics collects request metrics.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// IdempotencyMiddleware ensures mutating requests with an Idempotency-Key header are executed at-most-once.
func IdempotencyMiddleware(idempRepo redisRepo.IdempotencyStore, defaultTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		idempKey := c.GetHeader("Idempotency-Key")
		if idempKey == "" {
			response.AbortWithError(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", domain.ErrIdempotencyKeyMissing.Error())
			return
		}

		// Read and hash request body + path + method
		var bodyBytes []byte
		if c.Request.Body != nil {
			var err error
			bodyBytes, err = io.ReadAll(c.Request.Body)
			if err != nil {
				response.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read request body")
				return
			}
			// Re-populate body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		hasher := sha256.New()
		hasher.Write([]byte(c.Request.Method + ":" + c.Request.URL.Path + ":"))
		hasher.Write(bodyBytes)
		reqHash := hex.EncodeToString(hasher.Sum(nil))

		inProgressTTL := 30 * time.Second
		acquired, existing, err := idempRepo.TryAcquire(c.Request.Context(), idempKey, reqHash, inProgressTTL)
		if err != nil {
			if err == domain.ErrIdempotencyConflict {
				response.AbortWithError(c, http.StatusConflict, "CONFLICT", domain.ErrIdempotencyConflict.Error())
				return
			}
			if err == domain.ErrIdempotencyPayloadMismatch {
				response.AbortWithError(c, http.StatusUnprocessableEntity, "PAYLOAD_MISMATCH", domain.ErrIdempotencyPayloadMismatch.Error())
				return
			}
			response.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}

		if !acquired && existing != nil && existing.Status == redisRepo.IdempotencyStatusCompleted {
			// Request was already processed: replay cached response directly
			c.Header("X-Cache-Lookup", "HIT-IDEMPOTENCY")
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.String(existing.StatusCode, existing.ResponseBody)
			c.Abort()
			return
		}

		// Intercept the response so we can cache it upon completion
		writer := &responseBodyWriter{body: bytes.NewBuffer(nil), ResponseWriter: c.Writer}
		c.Writer = writer

		c.Next()

		statusCode := c.Writer.Status()
		if statusCode >= 200 && statusCode < 300 {
			// Save completed response in Redis with configured TTL
			_ = idempRepo.SaveCompleted(
				c.Request.Context(),
				idempKey,
				reqHash,
				statusCode,
				writer.body.Bytes(),
				defaultTTL,
			)
		} else {
			// Release lock on error so the client can retry
			_ = idempRepo.Release(c.Request.Context(), idempKey)
		}
	}
}
