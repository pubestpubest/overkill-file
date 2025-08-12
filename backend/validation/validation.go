package validation

import (
	"filebox/backend/models"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	if email == "" {
		return models.ValidationError{Field: "email", Message: "email is required"}
	}
	
	if _, err := mail.ParseAddress(email); err != nil {
		return models.ValidationError{Field: "email", Message: "invalid email format"}
	}
	
	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if password == "" {
		return models.ValidationError{Field: "password", Message: "password is required"}
	}
	
	if len(password) < 8 {
		return models.ValidationError{Field: "password", Message: "password must be at least 8 characters long"}
	}
	
	if len(password) > 128 {
		return models.ValidationError{Field: "password", Message: "password must be less than 128 characters"}
	}
	
	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)
	
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	
	if !hasUpper {
		return models.ValidationError{Field: "password", Message: "password must contain at least one uppercase letter"}
	}
	
	if !hasLower {
		return models.ValidationError{Field: "password", Message: "password must contain at least one lowercase letter"}
	}
	
	if !hasNumber {
		return models.ValidationError{Field: "password", Message: "password must contain at least one number"}
	}
	
	if !hasSpecial {
		return models.ValidationError{Field: "password", Message: "password must contain at least one special character"}
	}
	
	return nil
}

// ValidateFileName validates file name
func ValidateFileName(name string) error {
	if name == "" {
		return models.ValidationError{Field: "name", Message: "file name is required"}
	}
	
	if len(name) > 255 {
		return models.ValidationError{Field: "name", Message: "file name must be less than 255 characters"}
	}
	
	// Check for invalid characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return models.ValidationError{Field: "name", Message: "file name contains invalid characters"}
		}
	}
	
	// Check for reserved names (Windows)
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	// Remove extension and check base name
	baseName := name
	if dotIndex := strings.LastIndex(name, "."); dotIndex > 0 {
		baseName = name[:dotIndex]
	}
	upperName := strings.ToUpper(baseName)
	for _, reserved := range reservedNames {
		if upperName == reserved {
			return models.ValidationError{Field: "name", Message: "file name is reserved"}
		}
	}
	
	return nil
}

// ValidateFileSize validates file size (in bytes)
func ValidateFileSize(size int64) error {
	const maxSize = 100 * 1024 * 1024 // 100MB
	
	if size < 0 {
		return models.ValidationError{Field: "size", Message: "file size cannot be negative"}
	}
	
	if size > maxSize {
		return models.ValidationError{Field: "size", Message: "file size exceeds maximum allowed size (100MB)"}
	}
	
	return nil
}

// ValidateContentType validates content type
func ValidateContentType(contentType string) error {
	if contentType == "" {
		return nil // Content type is optional
	}
	
	// Basic MIME type validation
	mimeRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9!#$&\-\^_]*\/[a-zA-Z0-9][a-zA-Z0-9!#$&\-\^_.]*$`)
	if !mimeRegex.MatchString(contentType) {
		return models.ValidationError{Field: "content_type", Message: "invalid content type format"}
	}
	
	return nil
}

// ValidateTags validates file tags
func ValidateTags(tags models.Tags) error {
	if len(tags) > 10 {
		return models.ValidationError{Field: "tags", Message: "maximum 10 tags allowed"}
	}
	
	for i, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return models.ValidationError{Field: "tags", Message: "empty tags are not allowed"}
		}
		
		if len(tag) > 50 {
			return models.ValidationError{Field: "tags", Message: "tag length must be less than 50 characters"}
		}
		
		// Update the tag after trimming
		tags[i] = tag
	}
	
	return nil
}

// ValidateUser validates a user model
func ValidateUser(user *models.User) models.ValidationErrors {
	var errors models.ValidationErrors
	
	if err := ValidateEmail(user.Email); err != nil {
		if ve, ok := err.(models.ValidationError); ok {
			errors = append(errors, ve)
		}
	}
	
	return errors
}

// ValidateFile validates a file model
func ValidateFile(file *models.File) models.ValidationErrors {
	var errors models.ValidationErrors
	
	if err := ValidateFileName(file.Name); err != nil {
		if ve, ok := err.(models.ValidationError); ok {
			errors = append(errors, ve)
		}
	}
	
	if file.Size != nil {
		if err := ValidateFileSize(*file.Size); err != nil {
			if ve, ok := err.(models.ValidationError); ok {
				errors = append(errors, ve)
			}
		}
	}
	
	if file.ContentType != nil {
		if err := ValidateContentType(*file.ContentType); err != nil {
			if ve, ok := err.(models.ValidationError); ok {
				errors = append(errors, ve)
			}
		}
	}
	
	if err := ValidateTags(file.Tags); err != nil {
		if ve, ok := err.(models.ValidationError); ok {
			errors = append(errors, ve)
		}
	}
	
	return errors
}