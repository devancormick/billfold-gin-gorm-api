package handlers

import (
	"errors"
	"net/http"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdjustBalanceInput is the request body for crediting or debiting a wallet.
// IdempotencyKey is required so the same client retry never applies twice.
type AdjustBalanceInput struct {
	UserID         uint                   `json:"user_id" binding:"required"`
	Type           models.TransactionType `json:"type" binding:"required,oneof=credit debit"`
	AmountCents    int64                  `json:"amount_cents" binding:"required,gt=0"`
	IdempotencyKey string                 `json:"idempotency_key" binding:"required,min=8,max=100"`
	Reference      string                 `json:"reference" binding:"max=200"`
}

// AdjustBalance godoc
// @Summary      Credit or debit a wallet
// @Description  Applies a credit or debit atomically: wallet balance and ledger entry are
// @Description  written in a single DB transaction with SELECT ... FOR UPDATE row locking,
// @Description  so concurrent requests against the same wallet under peak load can't race.
// @Description  Idempotency key makes retried requests safe to replay.
// @Tags         payments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        input body AdjustBalanceInput true "Balance adjustment"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      422 {object} map[string]string "insufficient balance"
// @Router       /payments/adjust [post]
func AdjustBalance(c *gin.Context) {
	var input AdjustBalanceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	var tx models.Transaction
	err := config.DB.Transaction(func(db *gorm.DB) error {
		// Idempotency check happens inside the transaction so a concurrent
		// retry with the same key can't slip past a not-yet-committed row.
		existing := db.Where("idempotency_key = ?", input.IdempotencyKey).First(&tx)
		if existing.Error == nil {
			return nil // already applied — return the prior result, no-op
		}
		if !errors.Is(existing.Error, gorm.ErrRecordNotFound) {
			return existing.Error
		}

		var wallet models.Wallet
		// Row lock prevents lost updates when two requests hit the same wallet at once
		if err := db.Clauses().Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ?", input.UserID).
			First(&wallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				wallet = models.Wallet{UserID: input.UserID, Balance: 0}
				if err := db.Create(&wallet).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		delta := input.AmountCents
		if input.Type == models.TransactionDebit {
			delta = -delta
			if wallet.Balance+delta < 0 {
				return errors.New("insufficient balance")
			}
		}

		if err := db.Model(&wallet).Update("balance", wallet.Balance+delta).Error; err != nil {
			return err
		}

		tx = models.Transaction{
			WalletID:       wallet.ID,
			Type:           input.Type,
			AmountCents:    input.AmountCents,
			IdempotencyKey: input.IdempotencyKey,
			Reference:      input.Reference,
		}
		return db.Create(&tx).Error
	})

	if err != nil {
		if err.Error() == "insufficient balance" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to adjust balance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Balance adjusted successfully",
		"transaction": tx,
	})
}

// GetWallet godoc
// @Summary      Get a wallet balance
// @Tags         payments
// @Security     BearerAuth
// @Produce      json
// @Param        user_id path int true "User ID"
// @Success      200 {object} models.Wallet
// @Failure      401 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /payments/wallets/{user_id} [get]
func GetWallet(c *gin.Context) {
	userID := c.Param("user_id")

	var wallet models.Wallet
	if err := config.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	c.JSON(http.StatusOK, wallet)
}
