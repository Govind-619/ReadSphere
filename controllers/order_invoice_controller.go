package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

// DownloadInvoice generates and returns a PDF invoice for the order
func DownloadInvoice(c *gin.Context) {
	utils.LogInfo("Starting invoice download process")

	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("Unauthorized invoice download attempt - no user found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("User authenticated for invoice download: %s", user.Email)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID format in invoice download request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	utils.LogInfo("Processing invoice download for order ID: %d", orderID)

	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Preload("User").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.LogError("Order not found for invoice download - Order ID: %d, User ID: %d", orderID, user.ID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	utils.LogInfo("Found order for invoice generation - Order ID: %d", orderID)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Optional: Add logo (uncomment if logo.png exists)
	//pdf.ImageOptions("logo.png", 150, 5, 55, 0, false, gofpdf.ImageOptions{}, 0, "")

	// Store info
	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(100, 10, "Read Sphere")
	pdf.SetFont("Arial", "", 12)
	pdf.Ln(8)
	pdf.Cell(100, 8, "123 Main St, City, Country")
	pdf.Ln(8)
	pdf.Cell(100, 8, "Email: support@readsphere.com | Phone: +91-12345-67890")
	pdf.Ln(12)

	// Invoice title and order info
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(100, 10, "INVOICE")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(50, 8, "Order ID: "+strconv.Itoa(int(order.ID)))
	pdf.Cell(60, 8, "Order Date: "+order.CreatedAt.Format("2006-01-02 15:04:05"))
	pdf.Ln(8)
	pdf.Cell(50, 8, "Payment Method: "+order.PaymentMethod)
	pdf.Cell(60, 8, "Status: "+order.Status)
	pdf.Ln(8)

	// Customer and shipping info
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(100, 8, "Billed To:")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(100, 8, order.User.FirstName+" "+order.User.LastName)
	pdf.Ln(6)
	pdf.Cell(100, 8, order.User.Email)
	pdf.Ln(6)
	pdf.Cell(100, 8, "Phone: "+order.User.Phone)
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(100, 8, "Shipping Address:")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(100, 8, order.Address.Line1)
	pdf.Ln(6)
	if order.Address.Line2 != "" {
		pdf.Cell(100, 8, order.Address.Line2)
		pdf.Ln(6)
	}
	pdf.Cell(100, 8, order.Address.City+", "+order.Address.State+", "+order.Address.Country+" - "+order.Address.PostalCode)
	pdf.Ln(10)

	// Items table header
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(70, 8, "Book", "1", 0, "C", false, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "Price", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "Total", "1", 0, "C", false, 0, "")
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 12)
	for _, item := range order.OrderItems {
		pdf.CellFormat(70, 8, item.Book.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 8, strconv.Itoa(item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", item.Price), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", item.Total), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	// Summary section
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(120, 8, "Subtotal:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", order.TotalAmount), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(120, 8, "Discount:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", order.Discount), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(120, 10, "Grand Total:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.FinalTotal), "", 1, "R", false, 0, "")

	// Thank you note
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 12)
	pdf.Cell(0, 10, "Thank you for shopping with ReadSphere!")

	var buf bytes.Buffer
	_ = pdf.Output(&buf)
	utils.LogInfo("PDF invoice generated successfully for order ID: %d", orderID)

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=invoice.pdf")
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
	utils.LogInfo("Invoice download completed for order ID: %d", orderID)
}
