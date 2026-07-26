package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/handlers"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var paymentTestDBCounter int

// setupPaymentTestDB points the shared config.DB at a fresh, uniquely-named
// in-memory SQLite database for the duration of one test. Go runs all tests
// in a package within the same process, so a fixed DB name (even in-memory)
// would be shared across tests and let balances leak between them; a unique
// name per call guarantees isolation. Note: SQLite doesn't support
// SELECT ... FOR UPDATE, so these tests validate the idempotency and balance
// logic but not row-locking under real concurrency — that guarantee only
// holds against MariaDB/MySQL/Postgres in production.
func setupPaymentTestDB(t *testing.T) {
	t.Helper()
	paymentTestDBCounter++
	dsn := fmt.Sprintf("file:paymenttest%d?mode=memory&cache=shared", paymentTestDBCounter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Wallet{}, &models.Transaction{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	config.DB = db
}

func setupPaymentRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/payments/adjust", handlers.AdjustBalance)
	router.GET("/payments/wallets/:user_id", handlers.GetWallet)
	return router
}

func adjustBalance(t *testing.T, router *gin.Engine, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/payments/adjust", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdjustBalance_Credit(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	w := adjustBalance(t, router, map[string]interface{}{
		"user_id":         1,
		"type":            "credit",
		"amount_cents":    5000,
		"idempotency_key": "test-credit-001",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var wallet models.Wallet
	config.DB.Where("user_id = ?", 1).First(&wallet)
	assert.Equal(t, int64(5000), wallet.Balance)
}

func TestAdjustBalance_DebitInsufficientBalance(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	w := adjustBalance(t, router, map[string]interface{}{
		"user_id":         1,
		"type":            "debit",
		"amount_cents":    100,
		"idempotency_key": "test-debit-001",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "insufficient balance", response["error"])

	// wallet should not have been created/modified by a failed debit
	var count int64
	config.DB.Model(&models.Wallet{}).Where("user_id = ?", 1).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAdjustBalance_CreditThenDebit(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	adjustBalance(t, router, map[string]interface{}{
		"user_id":         1,
		"type":            "credit",
		"amount_cents":    5000,
		"idempotency_key": "test-cd-credit",
	})

	w := adjustBalance(t, router, map[string]interface{}{
		"user_id":         1,
		"type":            "debit",
		"amount_cents":    2000,
		"idempotency_key": "test-cd-debit",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var wallet models.Wallet
	config.DB.Where("user_id = ?", 1).First(&wallet)
	assert.Equal(t, int64(3000), wallet.Balance)
}

func TestAdjustBalance_IdempotencyKeyPreventsDoubleApply(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	payload := map[string]interface{}{
		"user_id":         1,
		"type":            "credit",
		"amount_cents":    5000,
		"idempotency_key": "test-idem-001",
	}

	w1 := adjustBalance(t, router, payload)
	assert.Equal(t, http.StatusOK, w1.Code)

	// replay the exact same request (simulates a client retry after a timeout)
	w2 := adjustBalance(t, router, payload)
	assert.Equal(t, http.StatusOK, w2.Code)

	var wallet models.Wallet
	config.DB.Where("user_id = ?", 1).First(&wallet)
	assert.Equal(t, int64(5000), wallet.Balance, "balance must not double-apply on a replayed idempotency key")

	var txCount int64
	config.DB.Model(&models.Transaction{}).Where("idempotency_key = ?", "test-idem-001").Count(&txCount)
	assert.Equal(t, int64(1), txCount, "only one ledger row should exist for a replayed key")
}

func TestAdjustBalance_ValidationErrors(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	t.Run("missing idempotency key", func(t *testing.T) {
		w := adjustBalance(t, router, map[string]interface{}{
			"user_id":      1,
			"type":         "credit",
			"amount_cents": 100,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("zero amount rejected", func(t *testing.T) {
		w := adjustBalance(t, router, map[string]interface{}{
			"user_id":         1,
			"type":            "credit",
			"amount_cents":    0,
			"idempotency_key": "test-zero-001",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid type rejected", func(t *testing.T) {
		w := adjustBalance(t, router, map[string]interface{}{
			"user_id":         1,
			"type":            "transfer",
			"amount_cents":    100,
			"idempotency_key": "test-badtype-001",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetWallet_NotFound(t *testing.T) {
	setupPaymentTestDB(t)
	router := setupPaymentRouter()

	req, _ := http.NewRequest("GET", "/payments/wallets/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
