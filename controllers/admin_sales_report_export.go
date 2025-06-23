package controllers

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"github.com/tealeg/xlsx"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
)

// Admin: Download sales report as Excel
func DownloadSalesReportExcel(c *gin.Context) {
	utils.LogInfo("DownloadSalesReportExcel called")

	period := c.DefaultQuery("period", "day")
	utils.LogDebug("Generating Excel report for period: %s", period)

	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	case "week":
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		startDate = endDate.AddDate(0, 0, -6)
		startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	default:
		utils.LogError("Invalid period specified: %s", period)
		utils.BadRequest(c, "Invalid period", "Period must be day, week, or month")
		return
	}

	var orders []models.Order
	query := config.DB.Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Preload("User").
		Preload("OrderItems.Book").
		Order("created_at DESC")
	if err := query.Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders: %v", err)
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders for Excel report", len(orders))

	// --- Calculate summary ---
	var summary struct {
		TotalSales      int
		TotalRevenue    float64
		TotalItems      int
		TotalCustomers  int
		TotalDiscounts  float64
		TotalRefunds    float64
		NetRevenue      float64
		AverageOrderVal float64
	}
	customerSet := make(map[uint]bool)
	for _, order := range orders {
		summary.TotalSales++
		summary.TotalRevenue += order.TotalAmount
		summary.TotalDiscounts += order.Discount + order.CouponDiscount
		customerSet[order.UserID] = true
		for _, item := range order.OrderItems {
			summary.TotalItems += item.Quantity
		}
		if order.Status == models.OrderStatusRefunded || order.Status == models.OrderStatusReturnCompleted {
			summary.TotalRefunds += order.RefundAmount
		}
	}
	summary.TotalCustomers = len(customerSet)
	if summary.TotalSales > 0 {
		summary.AverageOrderVal = math.Round((summary.TotalRevenue/float64(summary.TotalSales))*100) / 100
	}
	summary.NetRevenue = math.Round((summary.TotalRevenue-summary.TotalDiscounts-summary.TotalRefunds)*100) / 100
	summary.TotalRevenue = math.Round(summary.TotalRevenue*100) / 100
	summary.TotalDiscounts = math.Round(summary.TotalDiscounts*100) / 100
	summary.TotalRefunds = math.Round(summary.TotalRefunds*100) / 100

	// --- Excel Generation ---
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sales Report")
	if err != nil {
		utils.LogError("Failed to create Excel sheet: %v", err)
		utils.InternalServerError(c, "Failed to create Excel sheet", err.Error())
		return
	}
	utils.LogDebug("Created Excel sheet for sales report")

	// Company details
	companyRow := sheet.AddRow()
	companyRow.AddCell().SetString("READSPHERE - Sales Report")
	companyRow = sheet.AddRow()
	companyRow.AddCell().SetString("123 Book Street")
	companyRow = sheet.AddRow()
	companyRow.AddCell().SetString("Bookland, BK 12345")
	companyRow = sheet.AddRow()
	companyRow.AddCell().SetString("Email: support@readsphere.com")
	companyRow = sheet.AddRow()
	companyRow.AddCell().SetString("Phone: +1 234-567-8900")
	companyRow = sheet.AddRow()
	companyRow.AddCell().SetString("Period: " + strings.ToUpper(period) + " | " + startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02"))
	sheet.AddRow() // spacing

	// Table headers
	headers := []string{"Order ID", "User ID", "User Name", "Date", "Items", "Total", "Discount", "Net Amount", "Payment Mode", "Status"}
	headerRow := sheet.AddRow()
	for _, h := range headers {
		cell := headerRow.AddCell()
		cell.SetString(h)
		style := xlsx.NewStyle()
		font := xlsx.DefaultFont()
		font.Bold = true
		style.Font = *font
		cell.SetStyle(style)
	}

	// Table rows
	for _, order := range orders {
		row := sheet.AddRow()
		row.AddCell().SetInt(int(order.ID))
		row.AddCell().SetInt(int(order.User.ID))
		row.AddCell().SetString(order.User.Username)
		row.AddCell().SetString(order.CreatedAt.Format("2006-01-02 15:04"))
		row.AddCell().SetInt(len(order.OrderItems))
		row.AddCell().SetFloat(order.TotalAmount)
		row.AddCell().SetFloat(order.Discount + order.CouponDiscount)
		row.AddCell().SetFloat(order.TotalAmount - order.Discount - order.CouponDiscount)
		row.AddCell().SetString(order.PaymentMethod)
		row.AddCell().SetString(order.Status)
	}

	sheet.AddRow() // spacing

	// --- Summary Section ---
	summaryRow := sheet.AddRow()
	summaryRow.AddCell().SetString("Summary")
	style := xlsx.NewStyle()
	font := xlsx.DefaultFont()
	font.Bold = true
	style.Font = *font
	summaryRow.Cells[0].SetStyle(style)

	summaryData := [][]string{
		{"Total Sales", fmt.Sprintf("%d", summary.TotalSales)},
		{"Total Revenue", fmt.Sprintf("%.2f", summary.TotalRevenue)},
		{"Total Items", fmt.Sprintf("%d", summary.TotalItems)},
		{"Total Customers", fmt.Sprintf("%d", summary.TotalCustomers)},
		{"Total Discounts", fmt.Sprintf("%.2f", summary.TotalDiscounts)},
		{"Total Refunds", fmt.Sprintf("%.2f", summary.TotalRefunds)},
		{"Net Revenue", fmt.Sprintf("%.2f", summary.NetRevenue)},
		{"Avg. Order Value", fmt.Sprintf("%.2f", summary.AverageOrderVal)},
	}
	for _, data := range summaryData {
		row := sheet.AddRow()
		row.AddCell().SetString(data[0])
		row.AddCell().SetString(data[1])
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=sales_report_%s.xlsx", period))
	if err := file.Write(c.Writer); err != nil {
		utils.LogError("Failed to write Excel file: %v", err)
		utils.InternalServerError(c, "Failed to write Excel file", err.Error())
		return
	}
	utils.LogInfo("Successfully generated Excel report for period %s", period)
}

// Admin: Download sales report as PDF
func DownloadSalesReportPDF(c *gin.Context) {
	utils.LogInfo("DownloadSalesReportPDF called")

	period := c.DefaultQuery("period", "day")
	utils.LogDebug("Generating PDF report for period: %s", period)

	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	case "week":
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		startDate = endDate.AddDate(0, 0, -6)
		startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	default:
		utils.LogError("Invalid period specified: %s", period)
		utils.BadRequest(c, "Invalid period", "Period must be day, week, or month")
		return
	}

	var orders []models.Order
	query := config.DB.Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Preload("User").
		Preload("OrderItems.Book").
		Order("created_at DESC")
	if err := query.Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders: %v", err)
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders for PDF report", len(orders))

	// --- Calculate summary ---
	var summary struct {
		TotalSales      int
		TotalRevenue    float64
		TotalItems      int
		TotalCustomers  int
		TotalDiscounts  float64
		TotalRefunds    float64
		NetRevenue      float64
		AverageOrderVal float64
	}
	customerSet := make(map[uint]bool)
	for _, order := range orders {
		summary.TotalSales++
		summary.TotalRevenue += order.TotalAmount
		summary.TotalDiscounts += order.Discount + order.CouponDiscount
		customerSet[order.UserID] = true
		for _, item := range order.OrderItems {
			summary.TotalItems += item.Quantity
		}
		if order.Status == models.OrderStatusRefunded || order.Status == models.OrderStatusReturnCompleted {
			summary.TotalRefunds += order.RefundAmount
		}
	}
	summary.TotalCustomers = len(customerSet)
	if summary.TotalSales > 0 {
		summary.AverageOrderVal = math.Round((summary.TotalRevenue/float64(summary.TotalSales))*100) / 100
	}
	summary.NetRevenue = math.Round((summary.TotalRevenue-summary.TotalDiscounts-summary.TotalRefunds)*100) / 100
	summary.TotalRevenue = math.Round(summary.TotalRevenue*100) / 100
	summary.TotalDiscounts = math.Round(summary.TotalDiscounts*100) / 100
	summary.TotalRefunds = math.Round(summary.TotalRefunds*100) / 100

	// --- PDF Generation ---
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()

	// Add title
	pdf.SetFont("Arial", "B", 20)
	pdf.Cell(0, 12, "READSPHERE - Sales Report")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, "Online Book Store")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Period: "+strings.ToUpper(period)+" | "+startDate.Format("2006-01-02")+" to "+endDate.Format("2006-01-02"))
	pdf.Ln(10)

	// Add company details
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, "123 Book Street")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Bookland, BK 12345")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Email: support@readsphere.com")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Phone: +1 234-567-8900")
	pdf.Ln(10)

	// Table headers
	headers := []string{"Order ID", "User ID", "User Name", "Date", "Items", "Total", "Discount", "Net Amount", "Payment Mode", "Status"}
	colWidths := []float64{20, 20, 40, 32, 15, 25, 25, 30, 30, 30}
	pdf.SetFont("Arial", "B", 11)
	for i, h := range headers {
		pdf.SetFillColor(200, 200, 200)
		pdf.CellFormat(colWidths[i], 9, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 10)
	fill := false
	for _, order := range orders {
		pdf.SetFillColor(245, 245, 245)
		if fill {
			pdf.SetFillColor(230, 240, 255)
		}
		fill = !fill
		pdf.CellFormat(colWidths[0], 8, fmt.Sprintf("%d", order.ID), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[1], 8, fmt.Sprintf("%d", order.User.ID), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[2], 8, order.User.Username, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[3], 8, order.CreatedAt.Format("2006-01-02 15:04"), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[4], 8, fmt.Sprintf("%d", len(order.OrderItems)), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[5], 8, fmt.Sprintf("%.2f", order.TotalAmount), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(colWidths[6], 8, fmt.Sprintf("%.2f", order.Discount+order.CouponDiscount), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(colWidths[7], 8, fmt.Sprintf("%.2f", order.TotalAmount-order.Discount-order.CouponDiscount), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(colWidths[8], 8, order.PaymentMethod, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[9], 8, order.Status, "1", 0, "C", fill, 0, "")
		pdf.Ln(-1)
	}

	// --- Summary Section ---
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 13)
	pdf.SetFillColor(220, 230, 250)
	pdf.CellFormat(70, 10, "Summary", "1", 0, "C", true, 0, "")
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 11)
	pdf.SetFillColor(255, 255, 255)
	pdf.CellFormat(50, 8, "Total Sales", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%d", summary.TotalSales), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Total Revenue", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%.2f", summary.TotalRevenue), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Total Items", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%d", summary.TotalItems), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Total Customers", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%d", summary.TotalCustomers), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Total Discounts", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%.2f", summary.TotalDiscounts), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Total Refunds", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%.2f", summary.TotalRefunds), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Net Revenue", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%.2f", summary.NetRevenue), "1", 0, "R", false, 0, "")
	pdf.Ln(-1)
	pdf.CellFormat(50, 8, "Avg. Order Value", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 8, fmt.Sprintf("%.2f", summary.AverageOrderVal), "1", 0, "R", false, 0, "")

	// Set headers and write file
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=sales_report_%s.pdf", period))
	if err := pdf.Output(c.Writer); err != nil {
		utils.LogError("Failed to write PDF file: %v", err)
		utils.InternalServerError(c, "Failed to write PDF file", err.Error())
		return
	}
	utils.LogInfo("Successfully generated PDF report for period %s", period)
}
