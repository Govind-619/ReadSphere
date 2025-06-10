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

	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month
	utils.LogDebug("Generating Excel report for period: %s", period)

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = now.Truncate(24 * time.Hour)
		endDate = startDate.Add(24 * time.Hour)
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	case "week":
		startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	default:
		utils.LogError("Invalid period specified: %s", period)
		utils.BadRequest(c, "Invalid period", "Period must be day, week, or month")
		return
	}

	// Query orders within date range
	var orders []models.Order
	query := config.DB.Where("created_at >= ? AND created_at < ?", startDate, endDate).
		Preload("User").
		Preload("OrderItems.Book").
		Order("created_at DESC")

	if err := query.Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders: %v", err)
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders for Excel report", len(orders))

	// Create Excel file
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sales Report")
	if err != nil {
		utils.LogError("Failed to create Excel sheet: %v", err)
		utils.InternalServerError(c, "Failed to create Excel sheet", err.Error())
		return
	}
	utils.LogDebug("Created Excel sheet for sales report")

	// Add company details
	companyRow := sheet.AddRow()
	companyRow.AddCell().SetString("READSPHERE")
	companyRow.AddCell().SetString("Online Book Store")

	addressRow := sheet.AddRow()
	addressRow.AddCell().SetString("123 Book Street")
	addressRow.AddCell().SetString("Bookland, BK 12345")

	contactRow := sheet.AddRow()
	contactRow.AddCell().SetString("Email: support@readsphere.com")
	contactRow.AddCell().SetString("Phone: +1 234-567-8900")

	// Add empty row for spacing
	sheet.AddRow()

	// Add report info
	salesPersonRow := sheet.AddRow()
	salesPersonRow.AddCell().SetString("SALES PERSON")
	salesPersonRow.AddCell().SetString("DATE")
	salesPersonRow.AddCell().SetString("PERIOD")
	salesPersonRow.AddCell().SetString("DATE RANGE")

	infoRow := sheet.AddRow()
	infoRow.AddCell().SetString("Admin")
	infoRow.AddCell().SetString(time.Now().Format("01/02/06"))
	infoRow.AddCell().SetString(strings.ToUpper(period))
	infoRow.AddCell().SetString(fmt.Sprintf("%s to %s",
		startDate.Format("2006-01-02"),
		endDate.Add(-24*time.Hour).Format("2006-01-02")))

	// Add empty row for spacing
	sheet.AddRow()

	// Add table headers
	headerRow := sheet.AddRow()
	headers := []string{"ITEM NO", "ITEM NAME", "PRICE", "QTY", "TOTAL"}
	for _, header := range headers {
		cell := headerRow.AddCell()
		cell.SetString(header)
	}

	// Add sales data
	var totalAmount float64
	itemNo := 1

	for _, order := range orders {
		if order.Status == models.OrderStatusDelivered {
			for _, item := range order.OrderItems {
				row := sheet.AddRow()
				itemTotal := item.Price * float64(item.Quantity)

				// Add cells
				row.AddCell().SetString(fmt.Sprintf("%d", itemNo))
				row.AddCell().SetString(item.Book.Name)
				row.AddCell().SetFloat(item.Price)
				row.AddCell().SetInt(item.Quantity)
				row.AddCell().SetFloat(itemTotal)

				totalAmount += itemTotal
				itemNo++
			}
		}
	}
	utils.LogDebug("Added %d items to Excel report with total amount: %.2f", itemNo-1, totalAmount)

	// Add empty row
	sheet.AddRow()

	// Add total
	totalRow := sheet.AddRow()
	totalRow.AddCell().SetString("SALES TOTAL")
	totalRow.AddCell().SetFloat(totalAmount)

	// Set headers and write file
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

	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month
	utils.LogDebug("Generating PDF report for period: %s", period)

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {