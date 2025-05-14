package utils

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"gorm.io/gorm"
)

// GetOrCreateWallet retrieves or creates a wallet for a user
func GetOrCreateWallet(userID uint) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := config.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			wallet = models.Wallet{
				UserID:  userID,
				Balance: 0,
			}
			if err := config.DB.Create(&wallet).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return &wallet, nil
}

// CreateWalletTransaction creates a new wallet transaction
func CreateWalletTransaction(walletID uint, amount float64, transactionType string, description string, orderID *uint, reference string) (*models.WalletTransaction, error) {
	transaction := models.WalletTransaction{
		WalletID:    walletID,
		Amount:      amount,
		Type:        transactionType,
		Description: description,
		OrderID:     orderID,
		Reference:   reference,
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		return nil, err
	}

	return &transaction, nil
}

// UpdateWalletBalance updates the wallet balance based on transaction type
func UpdateWalletBalance(walletID uint, amount float64, transactionType string) error {
	var wallet models.Wallet
	if err := config.DB.First(&wallet, walletID).Error; err != nil {
		return err
	}

	if transactionType == models.TransactionTypeDebit {
		wallet.Balance -= amount
	} else {
		wallet.Balance += amount
	}

	return config.DB.Save(&wallet).Error
}
