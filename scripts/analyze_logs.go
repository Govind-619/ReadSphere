package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type LogStats struct {
	TotalErrors          int
	LoginSuccess         int
	LoginFailures        int
	OTPSuccess           int
	OTPFailures          int
	SQLInjectionAttempts int
	XSSAttempts          int
	FailedRequests       int
	UserActivities       map[string]int
	ErrorPatterns        map[string]int
}

func main() {
	// Get today's date for log file names
	today := time.Now().Format("2006-01-02")
	logDir := "./logs" // Changed from "../logs" to "./logs"

	// Initialize stats
	stats := &LogStats{
		UserActivities: make(map[string]int),
		ErrorPatterns:  make(map[string]int),
	}

	// Analyze error logs
	analyzeErrorLogs(filepath.Join(logDir, fmt.Sprintf("error-%s.log", today)), stats)

	// Analyze info logs
	analyzeInfoLogs(filepath.Join(logDir, fmt.Sprintf("info-%s.log", today)), stats)

	// Print report
	printReport(stats)
}

func analyzeErrorLogs(logFile string, stats *LogStats) {
	file, err := os.Open(logFile)
	if err != nil {
		fmt.Printf("Error opening log file %s: %v\n", logFile, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		stats.TotalErrors++

		// Count login failures
		if strings.Contains(line, "Login attempt failed") {
			stats.LoginFailures++
			extractUserActivity(line, stats)
		}

		// Count OTP failures
		if strings.Contains(line, "OTP verification failed") {
			stats.OTPFailures++
			extractUserActivity(line, stats)
		}

		// Count security attempts
		if strings.Contains(line, "SQL injection attempt") {
			stats.SQLInjectionAttempts++
		}
		if strings.Contains(line, "XSS attempt") {
			stats.XSSAttempts++
		}

		// Extract error patterns
		extractErrorPattern(line, stats)
	}
}

func analyzeInfoLogs(logFile string, stats *LogStats) {
	file, err := os.Open(logFile)
	if err != nil {
		fmt.Printf("Error opening log file %s: %v\n", logFile, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Count successful logins
		if strings.Contains(line, "User logged in successfully") {
			stats.LoginSuccess++
			extractUserActivity(line, stats)
		}

		// Count successful OTP verifications
		if strings.Contains(line, "User registration completed successfully") {
			stats.OTPSuccess++
			extractUserActivity(line, stats)
		}
	}
}

func extractUserActivity(line string, stats *LogStats) {
	// Extract email from log line
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	if email := emailRegex.FindString(line); email != "" {
		stats.UserActivities[email]++
	}
}

func extractErrorPattern(line string, stats *LogStats) {
	// Extract the main error message
	parts := strings.Split(line, ":")
	if len(parts) > 1 {
		errorMsg := strings.TrimSpace(parts[1])
		stats.ErrorPatterns[errorMsg]++
	}
}

func printReport(stats *LogStats) {
	fmt.Println("\n=== Log Analysis Report ===")
	fmt.Println("Generated:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("\n1. Authentication Statistics:")
	fmt.Printf("   Successful Logins: %d\n", stats.LoginSuccess)
	fmt.Printf("   Failed Logins: %d\n", stats.LoginFailures)
	fmt.Printf("   Successful OTP Verifications: %d\n", stats.OTPSuccess)
	fmt.Printf("   Failed OTP Verifications: %d\n", stats.OTPFailures)

	fmt.Println("\n2. Security Incidents:")
	fmt.Printf("   SQL Injection Attempts: %d\n", stats.SQLInjectionAttempts)
	fmt.Printf("   XSS Attempts: %d\n", stats.XSSAttempts)

	fmt.Println("\n3. Error Statistics:")
	fmt.Printf("   Total Errors: %d\n", stats.TotalErrors)

	fmt.Println("\n4. Most Active Users:")
	printTopUsers(stats.UserActivities, 5)

	fmt.Println("\n5. Most Common Errors:")
	printTopErrors(stats.ErrorPatterns, 5)
}

func printTopUsers(users map[string]int, limit int) {
	type userActivity struct {
		email string
		count int
	}

	var activities []userActivity
	for email, count := range users {
		activities = append(activities, userActivity{email, count})
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].count > activities[j].count
	})

	for i, activity := range activities {
		if i >= limit {
			break
		}
		fmt.Printf("   %s: %d activities\n", activity.email, activity.count)
	}
}

func printTopErrors(errors map[string]int, limit int) {
	type errorCount struct {
		error string
		count int
	}

	var errorList []errorCount
	for err, count := range errors {
		errorList = append(errorList, errorCount{err, count})
	}

	sort.Slice(errorList, func(i, j int) bool {
		return errorList[i].count > errorList[j].count
	})

	for i, err := range errorList {
		if i >= limit {
			break
		}
		fmt.Printf("   %s: %d occurrences\n", err.error, err.count)
	}
}
