package controllers

import (
	"fmt"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// Helper function to get or create a wallet for a user
func getOrCreateWallet(userID uint) (*models.Wallet, error) {
	var wallet models.Wallet
	err := config.DB.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		// Wallet doesn't exist, create one
		wallet = models.Wallet{
			UserID:  userID,
			Balance: 0,
		}
		if err := config.DB.Create(&wallet).Error; err != nil {
			return nil, err
		}
	}
	return &wallet, nil
}

// Helper function to create a wallet transaction
func createWalletTransaction(walletID uint, amount float64, transactionType string, description string, orderID *uint, reference string) (*models.WalletTransaction, error) {
	transaction := models.WalletTransaction{
		WalletID:    walletID,
		Amount:      amount,
		Type:        transactionType,
		Description: description,
		OrderID:     orderID,
		Reference:   reference,
		Status:      models.TransactionStatusCompleted,
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		return nil, err
	}

	return &transaction, nil
}

// Helper function to update wallet balance
func updateWalletBalance(walletID uint, amount float64, transactionType string) error {
	var wallet models.Wallet
	if err := config.DB.First(&wallet, walletID).Error; err != nil {
		return err
	}

	if transactionType == models.TransactionTypeCredit {
		wallet.Balance += amount
	} else if transactionType == models.TransactionTypeDebit {
		if wallet.Balance < amount {
			return fmt.Errorf("insufficient balance")
		}
		wallet.Balance -= amount
	}

	if err := config.DB.Save(&wallet).Error; err != nil {
		return err
	}

	return nil
}
