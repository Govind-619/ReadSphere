package controllers

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
	"github.com/jung-kurt/gofpdf"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// Admin: Download sales report as Excel
func DownloadSalesReportExcel(c *gin.Context) {
	// Parse filters (reuse GenerateSalesReport logic)
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	period := c.DefaultQuery("period", "custom")
	var startDate, endDate time.Time
	var err error
	if period != "custom" {
		today := time.Now()
		switch period {
		case "day":
			startDate = today.Truncate(24 * time.Hour)
			endDate = startDate.Add(24 * time.Hour)
		case "week":
			weekday := int(today.Weekday())
			if weekday == 0 { weekday = 7 }
			startDate = today.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
			endDate = startDate.AddDate(0, 0, 7)
		case "month":
			startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
			endDate = startDate.AddDate(0, 1, 0)
		}
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.String(400, "Invalid start_date")
			return
		}
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.String(400, "Invalid end_date")
			return
		}
	}
	db := config.DB
	var orders []models.Order
	db = db.Where("created_at >= ? AND created_at < ?", startDate, endDate).Preload("User").Preload("OrderItems.Book")
	db.Find(&orders)
	// Sales summary
	totalSalesCount := len(orders)
	totalOrderAmount := 0.0
	totalDiscount := 0.0
	totalCoupon := 0.0
	customerSet := make(map[uint]bool)
	orderStatusCount := map[string]int{"Completed": 0, "Cancelled": 0, "Returned": 0}
	refundTotal := 0.0
	productSales := make(map[string]int)
	customerAmount := make(map[uint]float64)
	for _, order := range orders {
		totalOrderAmount += order.TotalAmount
		totalDiscount += order.Discount
		totalCoupon += order.CouponDiscount
		customerSet[order.UserID] = true
		if order.Status == models.OrderStatusDelivered {
			orderStatusCount["Completed"]++
		} else if order.Status == models.OrderStatusCancelled {
			orderStatusCount["Cancelled"]++
		} else if order.Status == models.OrderStatusRefunded || order.Status == models.OrderStatusReturnCompleted {
			orderStatusCount["Returned"]++
			refundTotal += order.RefundAmount
		}
		customerAmount[order.UserID] += order.TotalAmount
		for _, item := range order.OrderItems {
			productSales[item.Book.Name] += item.Quantity
		}
	}
	// Top/Bottom 5 products
	type prodStat struct{ Name string; Qty int }
	var topProducts, bottomProducts []prodStat
	for name, qty := range productSales {
		topProducts = append(topProducts, prodStat{name, qty})
		bottomProducts = append(bottomProducts, prodStat{name, qty})
	}
	sort.Slice(topProducts, func(i, j int) bool { return topProducts[i].Qty > topProducts[j].Qty })
	sort.Slice(bottomProducts, func(i, j int) bool { return bottomProducts[i].Qty < bottomProducts[j].Qty })
	if len(topProducts) > 5 { topProducts = topProducts[:5] }
	if len(bottomProducts) > 5 { bottomProducts = bottomProducts[:5] }
	// Customer insights
	newCustomerCount := 0
	for id := range customerSet {
		var user models.User
		db.First(&user, id)
		if user.CreatedAt.After(startDate) && user.CreatedAt.Before(endDate) {
			newCustomerCount++
		}
	}
	returningCustomerCount := len(customerSet) - newCustomerCount
	// Top customers
	type custStat struct{ Name string; Amount float64 }
	var topCustomers []custStat
	for id, amt := range customerAmount {
		var user models.User
		db.First(&user, id)
		topCustomers = append(topCustomers, custStat{user.Username, amt})
	}
	sort.Slice(topCustomers, func(i, j int) bool { return topCustomers[i].Amount > topCustomers[j].Amount })
	if len(topCustomers) > 5 { topCustomers = topCustomers[:5] }
	// Excel generation
	file := xlsx.NewFile()
	sheet, _ := file.AddSheet("Sales Report of ReadSphere")
	// Title
	row := sheet.AddRow()
	row.AddCell().SetString("Sales Report of ReadSphere")
	row = sheet.AddRow()
	row.AddCell().SetString("Duration: " + period + ", Range: " + startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02"))
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Sales summary
	row = sheet.AddRow()
	row.AddCell().SetString("Summary")
	row = sheet.AddRow()
	row.AddCell().SetString("Total Sales Revenue: ")
	row.AddCell().SetFloat(math.Round(totalOrderAmount*100)/100)
	row = sheet.AddRow()
	row.AddCell().SetString("Number of Orders: ")
	row.AddCell().SetInt(totalSalesCount)
	row = sheet.AddRow()
	row.AddCell().SetString("Average Order Value: ")
	row.AddCell().SetFloat(math.Round((totalOrderAmount/float64(totalSalesCount))*100)/100)
	row = sheet.AddRow()
	row.AddCell().SetString("Number of Customers: ")
	row.AddCell().SetInt(len(customerSet))
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Product performance
	row = sheet.AddRow()
	row.AddCell().SetString("Top 5 Best-Selling Products:")
	for _, p := range topProducts {
		row = sheet.AddRow()
		row.AddCell().SetString(p.Name)
		row.AddCell().SetInt(p.Qty)
	}
	row = sheet.AddRow()
	row.AddCell().SetString("Top 5 Least-Selling Products:")
	for _, p := range bottomProducts {
		row = sheet.AddRow()
		row.AddCell().SetString(p.Name)
		row.AddCell().SetInt(p.Qty)
	}
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Customer insights
	row = sheet.AddRow()
	row.AddCell().SetString("New vs Returning Customers:")
	row = sheet.AddRow()
	row.AddCell().SetString("New Customers")
	row.AddCell().SetInt(newCustomerCount)
	row = sheet.AddRow()
	row.AddCell().SetString("Returning Customers")
	row.AddCell().SetInt(returningCustomerCount)
	row = sheet.AddRow()
	row.AddCell().SetString("Top 5 Customers by Purchase Amount:")
	for _, cst := range topCustomers {
		row = sheet.AddRow()
		row.AddCell().SetString(cst.Name)
		row.AddCell().SetFloat(math.Round(cst.Amount*100)/100)
	}
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Financials
	row = sheet.AddRow()
	row.AddCell().SetString("Total Discounts Given: ")
	row.AddCell().SetFloat(math.Round(totalDiscount*100)/100)
	row = sheet.AddRow()
	row.AddCell().SetString("Total Refunds Issued: ")
	row.AddCell().SetFloat(math.Round(refundTotal*100)/100)
	row = sheet.AddRow()
	row.AddCell().SetString("Net Revenue: ")
	row.AddCell().SetFloat(math.Round((totalOrderAmount-totalDiscount-refundTotal)*100)/100)
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Order status
	row = sheet.AddRow()
	row.AddCell().SetString("Order Status Summary:")
	row = sheet.AddRow()
	row.AddCell().SetString("Completed Orders")
	row.AddCell().SetInt(orderStatusCount["Completed"])
	row = sheet.AddRow()
	row.AddCell().SetString("Cancelled Orders")
	row.AddCell().SetInt(orderStatusCount["Cancelled"])
	row = sheet.AddRow()
	row.AddCell().SetString("Returned Orders")
	row.AddCell().SetInt(orderStatusCount["Returned"])
	row = sheet.AddRow()
	row.AddCell().SetString("")
	// Set headers and write file
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=sales_report.xlsx")
	file.Write(c.Writer)
}

// Admin: Download sales report as PDF
func DownloadSalesReportPDF(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	period := c.DefaultQuery("period", "custom")
	var startDate, endDate time.Time
	var err error
	if period != "custom" {
		today := time.Now()
		switch period {
		case "day":
			startDate = today.Truncate(24 * time.Hour)
			endDate = startDate.Add(24 * time.Hour)
		case "week":
			weekday := int(today.Weekday())
			if weekday == 0 { weekday = 7 }
			startDate = today.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
			endDate = startDate.AddDate(0, 0, 7)
		case "month":
			startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
			endDate = startDate.AddDate(0, 1, 0)
		}
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.String(400, "Invalid start_date")
			return
		}
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.String(400, "Invalid end_date")
			return
		}
	}
	db := config.DB
	var orders []models.Order
	db = db.Where("created_at >= ? AND created_at < ?", startDate, endDate).Preload("User").Preload("OrderItems.Book")
	db.Find(&orders)
	totalSalesCount := len(orders)
	totalOrderAmount := 0.0
	totalDiscount := 0.0
	totalCoupon := 0.0
	customerSet := make(map[uint]bool)
	orderStatusCount := map[string]int{"Completed": 0, "Cancelled": 0, "Returned": 0}
	refundTotal := 0.0
	productSales := make(map[string]int)
	customerAmount := make(map[uint]float64)
	for _, order := range orders {
		totalOrderAmount += order.TotalAmount
		totalDiscount += order.Discount
		totalCoupon += order.CouponDiscount
		customerSet[order.UserID] = true
		if order.Status == models.OrderStatusDelivered {
			orderStatusCount["Completed"]++
		} else if order.Status == models.OrderStatusCancelled {
			orderStatusCount["Cancelled"]++
		} else if order.Status == models.OrderStatusRefunded || order.Status == models.OrderStatusReturnCompleted {
			orderStatusCount["Returned"]++
			refundTotal += order.RefundAmount
		}
		customerAmount[order.UserID] += order.TotalAmount
		for _, item := range order.OrderItems {
			productSales[item.Book.Name] += item.Quantity
		}
	}
	type prodStat struct{ Name string; Qty int }
	var topProducts, bottomProducts []prodStat
	for name, qty := range productSales {
		topProducts = append(topProducts, prodStat{name, qty})
		bottomProducts = append(bottomProducts, prodStat{name, qty})
	}
	sort.Slice(topProducts, func(i, j int) bool { return topProducts[i].Qty > topProducts[j].Qty })
	sort.Slice(bottomProducts, func(i, j int) bool { return bottomProducts[i].Qty < bottomProducts[j].Qty })
	if len(topProducts) > 5 { topProducts = topProducts[:5] }
	if len(bottomProducts) > 5 { bottomProducts = bottomProducts[:5] }
	newCustomerCount := 0
	for id := range customerSet {
		var user models.User
		db.First(&user, id)
		if user.CreatedAt.After(startDate) && user.CreatedAt.Before(endDate) {
			newCustomerCount++
		}
	}
	returningCustomerCount := len(customerSet) - newCustomerCount
	type custStat struct{ Name string; Amount float64 }
	var topCustomers []custStat
	for id, amt := range customerAmount {
		var user models.User
		db.First(&user, id)
		topCustomers = append(topCustomers, custStat{user.Username, amt})
	}
	sort.Slice(topCustomers, func(i, j int) bool { return topCustomers[i].Amount > topCustomers[j].Amount })
	if len(topCustomers) > 5 { topCustomers = topCustomers[:5] }
	// PDF generation
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Sales Report of ReadSphere")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, "Duration: "+period+", Range: "+startDate.Format("2006-01-02")+" to "+endDate.Format("2006-01-02"))
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Total Sales Revenue: %.2f", totalOrderAmount))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Number of Orders: %d", totalSalesCount))
	pdf.Ln(7)
	if totalSalesCount > 0 {
		pdf.Cell(0, 8, fmt.Sprintf("Average Order Value: %.2f", totalOrderAmount/float64(totalSalesCount)))
		pdf.Ln(7)
	}
	pdf.Cell(0, 8, fmt.Sprintf("Number of Customers: %d", len(customerSet)))
	pdf.Ln(7)
	pdf.Ln(3)
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Top 5 Best-Selling Products:")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	for _, p := range topProducts {
		pdf.Cell(0, 7, fmt.Sprintf("%s (%d)", p.Name, p.Qty))
		pdf.Ln(7)
	}
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Top 5 Least-Selling Products:")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	for _, p := range bottomProducts {
		pdf.Cell(0, 7, fmt.Sprintf("%s (%d)", p.Name, p.Qty))
		pdf.Ln(7)
	}
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Customer Insights")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 7, fmt.Sprintf("New Customers: %d", newCustomerCount))
	pdf.Ln(7)
	pdf.Cell(0, 7, fmt.Sprintf("Returning Customers: %d", returningCustomerCount))
	pdf.Ln(7)
	pdf.Cell(0, 8, "Top 5 Customers by Purchase Amount:")
	pdf.Ln(7)
	for _, cst := range topCustomers {
		pdf.Cell(0, 7, fmt.Sprintf("%s (%.2f)", cst.Name, cst.Amount))
		pdf.Ln(7)
	}
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Financials")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 7, fmt.Sprintf("Total Discounts Given: %.2f", totalDiscount))
	pdf.Ln(7)
	pdf.Cell(0, 7, fmt.Sprintf("Total Refunds Issued: %.2f", refundTotal))
	pdf.Ln(7)
	pdf.Cell(0, 7, fmt.Sprintf("Net Revenue: %.2f", totalOrderAmount-totalDiscount-refundTotal))
	pdf.Ln(7)
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(0, 8, "Order Status Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 7, fmt.Sprintf("Completed Orders: %d", orderStatusCount["Completed"]))
	pdf.Ln(7)
	pdf.Cell(0, 7, fmt.Sprintf("Cancelled Orders: %d", orderStatusCount["Cancelled"]))
	pdf.Ln(7)
	pdf.Cell(0, 7, fmt.Sprintf("Returned Orders: %d", orderStatusCount["Returned"]))
	pdf.Ln(7)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=sales_report.pdf")
	_ = pdf.Output(c.Writer)
}
