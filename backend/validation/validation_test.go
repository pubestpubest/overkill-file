package validation

import (
	"filebox/backend/models"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"test@example.com", false},
		{"user.name+tag@domain.co.uk", false},
		{"", true},
		{"invalid-email", true},
		{"@domain.com", true},
		{"user@", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"Password123!", false},
		{"StrongP@ss1", false},
		{"", true},
		{"short", true},
		{"nouppercase123!", true},
		{"NOLOWERCASE123!", true},
		{"NoNumbers!", true},
		{"NoSpecialChars123", true},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"document.pdf", false},
		{"my-file_v2.txt", false},
		{"", true},
		{"file/with/slash.txt", true},
		{"file\\with\\backslash.txt", true},
		{"file:with:colon.txt", true},
		{"CON", true},
		{"PRN.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileSize(t *testing.T) {
	tests := []struct {
		size    int64
		wantErr bool
	}{
		{1024, false},
		{50 * 1024 * 1024, false}, // 50MB
		{-1, true},
		{200 * 1024 * 1024, true}, // 200MB (exceeds 100MB limit)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := ValidateFileSize(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileSize(%d) error = %v, wantErr %v", tt.size, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		tags    models.Tags
		wantErr bool
	}{
		{models.Tags{"tag1", "tag2"}, false},
		{models.Tags{}, false},
		{models.Tags{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10"}, false},
		{models.Tags{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10", "tag11"}, true}, // Too many tags
		{models.Tags{"", "tag2"}, true}, // Empty tag
		{models.Tags{"this-is-a-very-long-tag-name-that-exceeds-the-fifty-character-limit"}, true}, // Tag too long
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := ValidateTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}