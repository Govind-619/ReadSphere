package utils

import (
	"fmt"
	"math"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

type CartDetails struct {
	OrderItems       []models.OrderItem
	Subtotal         float64
	ProductDiscount  float64
	CategoryDiscount float64
	CouponDiscount   float64
	CouponCode       string
	FinalTotal       float64
}

// GetCartDetails retrieves cart details with all calculations
func GetCartDetails(userID uint) (*CartDetails, error) {
	db := config.DB
	var cartItems []models.Cart
	if err := db.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch cart items: %v", err)
	}

	var details CartDetails
	for _, item := range cartItems {
		book, err := GetBookByIDForCart(item.BookID)
		if err != nil || book == nil {
			continue
		}

		// Get offer breakdown
		offerBreakdown, _ := GetOfferBreakdownForBook(book.ID, book.CategoryID)

		// Calculate discounts
		originalItemTotal := book.Price * float64(item.Quantity)
		productDiscount := (book.Price * offerBreakdown.ProductOfferPercent / 100) * float64(item.Quantity)
		categoryDiscount := (book.Price * offerBreakdown.CategoryOfferPercent / 100) * float64(item.Quantity)
		finalUnitPrice := book.Price - (book.Price * (offerBreakdown.ProductOfferPercent + offerBreakdown.CategoryOfferPercent) / 100)
		itemTotal := finalUnitPrice * float64(item.Quantity)

		details.OrderItems = append(details.OrderItems, models.OrderItem{
			BookID:   book.ID,
			Book:     *book,
			Quantity: item.Quantity,
			Price:    book.Price,
			Discount: productDiscount + categoryDiscount,
			Total:    itemTotal,
		})

		details.Subtotal += originalItemTotal
		details.ProductDiscount += productDiscount
		details.CategoryDiscount += categoryDiscount
	}

	// Get active coupon if any
	var activeUserCoupon models.UserActiveCoupon
	if err := db.Where("user_id = ?", userID).First(&activeUserCoupon).Error; err == nil {
		var coupon models.Coupon
		if err := db.Where("id = ?", activeUserCoupon.CouponID).First(&coupon).Error; err == nil {
			details.CouponCode = coupon.Code
			if coupon.Type == "percent" {
				details.CouponDiscount = (details.Subtotal * coupon.Value) / 100
				if details.CouponDiscount > coupon.MaxDiscount {
					details.CouponDiscount = coupon.MaxDiscount
				}
			} else {
				details.CouponDiscount = coupon.Value
			}
		}
	}

	// Calculate final total
	totalDiscount := details.ProductDiscount + details.CategoryDiscount + details.CouponDiscount
	details.FinalTotal = math.Round((details.Subtotal-totalDiscount)*100) / 100

	return &details, nil
}

// ProcessWalletPayment processes a wallet payment for an order
func ProcessWalletPayment(walletID uint, amount float64, orderID uint) error {
	reference := fmt.Sprintf("ORDER-%d", orderID)
	description := fmt.Sprintf("Payment for order #%d", orderID)

	_, err := createWalletTransaction(walletID, amount, models.TransactionTypeDebit, description, &orderID, reference)
	if err != nil {
		return fmt.Errorf("failed to create wallet transaction: %v", err)
	}

	if err := updateWalletBalance(walletID, amount, models.TransactionTypeDebit); err != nil {
		return fmt.Errorf("failed to update wallet balance: %v", err)
	}

	return nil
}

func createWalletTransaction(walletID uint, amount float64, transactionType string, description string, orderID *uint, reference string) (*models.WalletTransaction, error) {
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

func updateWalletBalance(walletID uint, amount float64, transactionType string) error {
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
