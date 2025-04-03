package utils

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// AllowedImageTypes defines the allowed image file extensions
var AllowedImageTypes = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

// ValidateImageFile checks if the uploaded file is a valid image
func ValidateImageFile(file *multipart.FileHeader) error {
	// Check file size (max 5MB)
	if file.Size > 5*1024*1024 {
		return fmt.Errorf("file size exceeds 5MB limit")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !AllowedImageTypes[ext] {
		return fmt.Errorf("invalid file type. Allowed types: jpg, jpeg, png, gif")
	}

	return nil
}

// SaveUploadedFile saves an uploaded file to the uploads directory
func SaveUploadedFile(file *multipart.FileHeader, uploadDir string) (string, error) {
	// Validate the file
	if err := ValidateImageFile(file); err != nil {
		return "", err
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := uuid.New().String() + ext
	filepath := filepath.Join(uploadDir, filename)

	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create uploads directory: %v", err)
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	// Create the destination file
	dst, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	// Copy the file content
	if _, err := dst.ReadFrom(src); err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	// Return the relative path
	return "uploads/" + filename, nil
}

// DeleteFile deletes a file from the filesystem
func DeleteFile(filepath string) error {
	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}
