package controllers

import (
	"fmt"
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
	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = now.Truncate(24 * time.Hour)
		endDate = startDate.Add(24 * time.Hour)
	case "week":
		startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	default:
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
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}

	// Create Excel file
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sales Report")
	if err != nil {
		utils.InternalServerError(c, "Failed to create Excel sheet", err.Error())
		return
	}

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
		utils.InternalServerError(c, "Failed to write Excel file", err.Error())
		return
	}
}

// Admin: Download sales report as PDF
func DownloadSalesReportPDF(c *gin.Context) {
	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = now.Truncate(24 * time.Hour)
		endDate = startDate.Add(24 * time.Hour)
	case "week":
		startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	default:
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
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Add current date at the top right
	pdf.SetFont("Arial", "I", 10)
	pdf.Cell(150, 8, "Generated on: "+time.Now().Format("January 2, 2006"))
	pdf.Ln(12)

	// Set font for company name
	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(0, 10, "READSPHERE")
	pdf.Ln(8)

	// Add company details
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, "Online Book Store")
	pdf.Ln(6)
	pdf.Cell(0, 8, "123 Book Street")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Bookland, BK 12345")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Email: support@readsphere.com")
	pdf.Ln(6)
	pdf.Cell(0, 8, "Phone: +1 234-567-8900")
	pdf.Ln(15)

	// Add report info
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 8, "SALES PERSON:")
	pdf.Cell(60, 8, "Admin")
	pdf.Cell(30, 8, "PERIOD:")
	pdf.Cell(0, 8, strings.ToUpper(period))
	pdf.Ln(8)

	pdf.Cell(40, 8, "DATE RANGE:")
	pdf.Cell(0, 8, fmt.Sprintf("%s to %s",
		startDate.Format("2006-01-02"),
		endDate.Add(-24*time.Hour).Format("2006-01-02")))
	pdf.Ln(15)

	// Add table headers
	pdf.SetFont("Arial", "B", 10)
	headers := []string{"ITEM NO", "DATE", "ITEM NAME", "PRICE", "QTY", "TOTAL"}
	colWidths := []float64{20, 25, 65, 25, 20, 25}

	// Draw header cells
	for i, header := range headers {
		pdf.Cell(colWidths[i], 8, header)
	}
	pdf.Ln(8)

	// Add sales data
	pdf.SetFont("Arial", "", 10)
	var totalAmount float64
	itemNo := 1

	for _, order := range orders {
		if order.Status == models.OrderStatusDelivered {
			for _, item := range order.OrderItems {
				itemTotal := item.Price * float64(item.Quantity)

				// Format numbers
				price := fmt.Sprintf("%.2f", item.Price)
				totalStr := fmt.Sprintf("%.2f", itemTotal)

				// Add row
				pdf.Cell(colWidths[0], 8, fmt.Sprintf("%d", itemNo))
				pdf.Cell(colWidths[1], 8, order.CreatedAt.Format("02/01/06"))
				pdf.Cell(colWidths[2], 8, item.Book.Name)
				pdf.Cell(colWidths[3], 8, price)
				pdf.Cell(colWidths[4], 8, fmt.Sprintf("%d", item.Quantity))
				pdf.Cell(colWidths[5], 8, totalStr)
				pdf.Ln(8)

				totalAmount += itemTotal
				itemNo++
			}
		}
	}

	// Add total with extra space
	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(120, 8, "")
	pdf.Cell(30, 8, "SALES TOTAL:")
	pdf.Cell(10, 8, "") // Add extra space
	pdf.Cell(0, 8, fmt.Sprintf("%.2f", totalAmount))

	// Set headers and output PDF
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=sales_report_%s.pdf", period))

	if err := pdf.Output(c.Writer); err != nil {
		utils.InternalServerError(c, "Failed to generate PDF", err.Error())
		return
	}
}
