package controllers

import (
	"fmt"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetWalletBalance returns the user's wallet balance
func GetWalletBalance(c *gin.Context) {
	utils.LogInfo("GetWalletBalance called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	utils.LogInfo("Processing wallet balance request for user ID: %d", user.ID)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.LogError("Failed to get wallet for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}
	utils.LogInfo("Successfully retrieved wallet balance for user ID: %d", user.ID)

	utils.Success(c, "Wallet balance retrieved successfully", gin.H{
		"balance": fmt.Sprintf("%.2f", wallet.Balance),
	})
}

// GetWalletTransactions returns the user's wallet transactions
func GetWalletTransactions(c *gin.Context) {
	utils.LogInfo("GetWalletTransactions called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	utils.LogInfo("Processing wallet transactions request for user ID: %d", user.ID)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.LogError("Failed to get wallet for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}

	// Get pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit
	utils.LogDebug("Pagination parameters - Page: %d, Limit: %d, Offset: %d", page, limit, offset)

	// Get transactions
	var transactions []models.WalletTransaction
	var total int64
	if err := config.DB.Model(&models.WalletTransaction{}).Where("wallet_id = ?", wallet.ID).Count(&total).Error; err != nil {
		utils.LogError("Failed to count transactions for wallet ID: %d: %v", wallet.ID, err)
		utils.InternalServerError(c, "Failed to count transactions", err.Error())
		return
	}
	utils.LogDebug("Found %d total transactions for wallet ID: %d", total, wallet.ID)

	if err := config.DB.Where("wallet_id = ?", wallet.ID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		utils.LogError("Failed to get transactions for wallet ID: %d: %v", wallet.ID, err)
		utils.InternalServerError(c, "Failed to get transactions", err.Error())
		return
	}
	utils.LogInfo("Successfully retrieved %d transactions for wallet ID: %d", len(transactions), wallet.ID)

	// Format transaction amounts
	formattedTransactions := make([]gin.H, len(transactions))
	for i, txn := range transactions {
		formattedTransactions[i] = gin.H{
			"id":          txn.ID,
			"amount":      fmt.Sprintf("%.2f", txn.Amount),
			"type":        txn.Type,
			"description": txn.Description,
			"reference":   txn.Reference,
			"created_at":  txn.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	utils.SuccessWithPagination(c, "Wallet transactions retrieved successfully", gin.H{
		"transactions": formattedTransactions,
		"wallet": gin.H{
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		},
	}, total, page, limit)
}

// ProcessOrderCancellation has been deprecated and merged into CancelOrder
func ProcessOrderCancellation(c *gin.Context) {
	utils.LogInfo("ProcessOrderCancellation called - Deprecated endpoint")
	utils.BadRequest(c, "This endpoint is deprecated. Please use /user/orders/:id/cancel instead", nil)
}

// Admin endpoint to approve return and process refund
