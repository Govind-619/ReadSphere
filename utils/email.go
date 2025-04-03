package utils

import (
	"fmt"
	"net/smtp"
	"os"

	"gopkg.in/gomail.v2"
)

// EmailConfig holds email configuration
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SendEmail sends an email using SMTP
func SendEmail(to, subject, body string) error {
	// Get SMTP configuration from environment variables
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Create email message
	message := fmt.Sprintf("Subject: %s\r\n"+
		"To: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", subject, to, body)

	// Connect to SMTP server
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	// Send email
	err := smtp.SendMail(addr, auth, smtpUsername, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

// SendOTP sends an OTP via email
func SendOTP(to, otp string) error {
	// Get email configuration from environment variables
	config := EmailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     587, // Default SMTP port
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", config.From)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Your ReadSphere Registration OTP")

	// Create email body
	body := fmt.Sprintf(`
		<h2>Welcome to ReadSphere!</h2>
		<p>Thank you for registering. Please use the following OTP to verify your email address:</p>
		<h1 style="color: #4CAF50; font-size: 32px; letter-spacing: 5px;">%s</h1>
		<p>This OTP will expire in 15 minutes.</p>
		<p>If you didn't request this OTP, please ignore this email.</p>
	`, otp)
	m.SetBody("text/html", body)

	// Create SMTP dialer
	d := gomail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	// Send email
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

// SendPasswordResetEmail sends a password reset email
func SendPasswordResetEmail(to, resetToken string) error {
	subject := "Password Reset Request"
	body := fmt.Sprintf(`
		<h2>Password Reset Request</h2>
		<p>You have requested to reset your password. Click the link below to proceed:</p>
		<p><a href="%s/reset-password?token=%s">Reset Password</a></p>
		<p>This link will expire in 1 hour.</p>
		<p>If you didn't request this reset, please ignore this email.</p>
	`, os.Getenv("FRONTEND_URL"), resetToken)

	return SendEmail(to, subject, body)
}
