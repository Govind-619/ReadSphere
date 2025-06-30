package models

import (
	"time"
)

// Order status constants
const (
	OrderStatusPlaced          = "Placed"
	OrderStatusProcessing      = "Processing"
	OrderStatusPaid            = "Paid"
	OrderStatusShipped         = "Shipped"
	OrderStatusDelivered       = "Delivered"
	OrderStatusCancelled       = "Cancelled"
	OrderStatusRefunded        = "Refunded"
	OrderStatusReturnRequested = "Return Requested"
	OrderStatusReturnApproved  = "Return Approved"
	OrderStatusReturnRejected  = "Return Rejected"
	OrderStatusReturnCompleted = "Return Completed"
)

// Order represents an order in the system
type Order struct {
	ID                          uint        `gorm:"primaryKey" json:"id"`
	UserID                      uint        `json:"user_id"`
	User                        User        `json:"user" gorm:"foreignKey:UserID"`
	AddressID                   uint        `json:"address_id"`
	Address                     Address     `json:"address" gorm:"foreignKey:AddressID"`
	TotalAmount                 float64     `json:"total_amount"`
	Discount                    float64     `json:"discount"`
	CouponDiscount              float64     `json:"coupon_discount"`
	CouponID                    uint        `json:"coupon_id"`
	CouponCode                  string      `json:"coupon_code"`
	FinalTotal                  float64     `json:"final_total"`
	DeliveryCharge              float64     `json:"delivery_charge" gorm:"default:0"`
	TotalWithDelivery           float64     `json:"total_with_delivery"`
	PaymentMethod               string      `json:"payment_method"`
	PaymentID                   string      `json:"payment_id"`
	RazorpayOrderID             string      `json:"razorpay_order_id"`
	RazorpayPaymentID           string      `json:"razorpay_payment_id"`
	RazorpaySignature           string      `json:"razorpay_signature"`
	Status                      string      `json:"status"`
	CancellationReason          string      `json:"cancellation_reason,omitempty"`
	ReturnReason                string      `json:"return_reason,omitempty"`
	ReturnRejectReason          string      `json:"return_reject_reason,omitempty"`
	RefundStatus                string      `json:"refund_status,omitempty"` // pending, completed, failed
	RefundAmount                float64     `json:"refund_amount,omitempty"`
	RefundedAt                  *time.Time  `json:"refunded_at,omitempty"`
	RefundedToWallet            bool        `json:"refunded_to_wallet,omitempty"`
	HasItemCancellationRequests bool        `json:"has_item_cancellation_requests,omitempty"`
	HasItemReturnRequests       bool        `json:"has_item_return_requests,omitempty"`
	CreatedAt                   time.Time   `json:"created_at"`
	UpdatedAt                   time.Time   `json:"updated_at"`
	OrderItems                  []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
	OriginalDetails             string      `json:"original_details" gorm:"type:json"`
}

type OrderItem struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	OrderID               uint       `json:"order_id"`
	BookID                uint       `json:"book_id"`
	Book                  Book       `json:"book"`
	Quantity              int        `json:"quantity"`
	Price                 float64    `json:"price"`
	Discount              float64    `json:"discount"`
	Total                 float64    `json:"total"`
	CancellationRequested bool       `json:"cancellation_requested"`
	CancellationReason    string     `json:"cancellation_reason"`
	CancellationStatus    string     `json:"cancellation_status"`
	ReturnRequested       bool       `json:"return_requested"`
	ReturnReason          string     `json:"return_reason"`
	ReturnStatus          string     `json:"return_status"`
	RefundStatus          string     `json:"refund_status"`
	RefundAmount          float64    `json:"refund_amount"`
	RefundedAt            *time.Time `json:"refunded_at"`
	StockRestored         bool       `json:"stock_restored" gorm:"default:false"`
	CouponDiscount        float64    `json:"coupon_discount"`
}
