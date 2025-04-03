package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	// InfoLogger logs informational messages
	InfoLogger *log.Logger
	// ErrorLogger logs error messages
	ErrorLogger *log.Logger
	// DebugLogger logs debug messages
	DebugLogger *log.Logger
)

// InitLogger initializes the loggers
func InitLogger() error {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log files with timestamp
	timestamp := time.Now().Format("2006-01-02")
	infoFile, err := os.OpenFile(
		filepath.Join(logsDir, fmt.Sprintf("info-%s.log", timestamp)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to open info log file: %v", err)
	}

	errorFile, err := os.OpenFile(
		filepath.Join(logsDir, fmt.Sprintf("error-%s.log", timestamp)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %v", err)
	}

	debugFile, err := os.OpenFile(
		filepath.Join(logsDir, fmt.Sprintf("debug-%s.log", timestamp)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %v", err)
	}

	// Initialize loggers
	InfoLogger = log.New(infoFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(errorFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	DebugLogger = log.New(debugFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

// LogInfo logs an informational message
func LogInfo(format string, v ...interface{}) {
	if InfoLogger != nil {
		InfoLogger.Printf(format, v...)
	}
}

// LogError logs an error message
func LogError(format string, v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Printf(format, v...)
	}
}

// LogDebug logs a debug message
func LogDebug(format string, v ...interface{}) {
	if DebugLogger != nil {
		DebugLogger.Printf(format, v...)
	}
}

// LogRequest logs HTTP request details
func LogRequest(method, path, ip string, status int, duration time.Duration) {
	LogInfo("Request: %s %s from %s - Status: %d - Duration: %v", method, path, ip, status, duration)
}

// LogErrorWithStack logs an error with stack trace
func LogErrorWithStack(err error, stack []byte) {
	if ErrorLogger != nil {
		ErrorLogger.Printf("Error: %v\nStack Trace:\n%s", err, stack)
	}
}
