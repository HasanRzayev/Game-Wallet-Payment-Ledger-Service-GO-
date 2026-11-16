package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"game-wallet-ledger/internal/domain"
	handler "game-wallet-ledger/internal/handler/http"
	"game-wallet-ledger/internal/service"
)

// MockWalletService implements service.WalletService for handler unit tests
type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) CreateWallet(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	args := m.Called(ctx, playerID, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Wallet), args.Error(1)
}

func (m *MockWalletService) GetWallet(ctx context.Context, walletID string) (*domain.Wallet, error) {
	args := m.Called(ctx, walletID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Wallet), args.Error(1)
}

func (m *MockWalletService) GetWalletByPlayerAndCurrency(ctx context.Context, playerID, currency string) (*domain.Wallet, error) {
	args := m.Called(ctx, playerID, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Wallet), args.Error(1)
}

func (m *MockWalletService) Deposit(ctx context.Context, req service.DepositRequest) (*service.TransactionResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TransactionResult), args.Error(1)
}

func (m *MockWalletService) Purchase(ctx context.Context, req service.PurchaseRequest) (*service.TransactionResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TransactionResult), args.Error(1)
}

func (m *MockWalletService) Refund(ctx context.Context, req service.RefundRequest) (*service.TransactionResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TransactionResult), args.Error(1)
}

func (m *MockWalletService) GetLedgerHistory(ctx context.Context, walletID string, limit, offset int) ([]*domain.LedgerEntry, error) {
	args := m.Called(ctx, walletID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LedgerEntry), args.Error(1)
}

func TestHandler_CreateWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockWalletService)
	h := handler.NewHandler(mockSvc, nil, nil, nil)

	r := gin.New()
	r.POST("/api/v1/wallets", h.CreateWallet)

	expectedWallet := &domain.Wallet{
		ID:        "w-123",
		PlayerID:  "player-xyz",
		Currency:  "USD",
		Balance:   0,
		Status:    domain.WalletStatusActive,
		CreatedAt: time.Now().UTC(),
	}

	mockSvc.On("CreateWallet", mock.Anything, "player-xyz", "USD").Return(expectedWallet, nil)

	reqBody := `{"player_id":"player-xyz","currency":"USD"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/wallets", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	mockSvc.AssertExpectations(t)
}

func TestHandler_GetWallet_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockWalletService)
	h := handler.NewHandler(mockSvc, nil, nil, nil)

	r := gin.New()
	r.GET("/api/v1/wallets/:id", h.GetWallet)

	mockSvc.On("GetWallet", mock.Anything, "non-existent").Return(nil, domain.ErrWalletNotFound)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/wallets/non-existent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp["success"].(bool))
	mockSvc.AssertExpectations(t)
}

func TestHandler_HealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handler.NewHandler(nil, nil, nil, nil)

	r := gin.New()
	r.GET("/health", h.HealthCheck)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "UP", resp["status"])
}
