package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// User represents a user in the system
type User struct {
	ID        int       `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// File represents a file with comprehensive metadata
type File struct {
	ID          int       `json:"id" db:"id"`
	UserID      int       `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Size        *int64    `json:"size,omitempty" db:"size"`
	ContentType *string   `json:"content_type,omitempty" db:"content_type"`
	Tags        Tags      `json:"tags" db:"tags"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Share represents a time-limited share link
type Share struct {
	ID        int       `json:"id" db:"id"`
	FileID    int       `json:"file_id" db:"file_id"`
	Token     string    `json:"token" db:"token"`
	Expires   time.Time `json:"expires" db:"expires"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Activity represents an audit log entry
type Activity struct {
	ID           int             `json:"id" db:"id"`
	UserID       *int            `json:"user_id,omitempty" db:"user_id"`
	Action       string          `json:"action" db:"action"`
	ResourceType *string         `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   *int            `json:"resource_id,omitempty" db:"resource_id"`
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// Tags represents a PostgreSQL text array
type Tags []string

// Scan implements the sql.Scanner interface for Tags
func (t *Tags) Scan(value interface{}) error {
	if value == nil {
		*t = Tags{}
		return nil
	}

	switch v := value.(type) {
	case pq.StringArray:
		*t = Tags(v)
		return nil
	case []string:
		*t = Tags(v)
		return nil
	case string:
		// Handle string representation like "{tag1,tag2}"
		if v == "" || v == "{}" {
			*t = Tags{}
			return nil
		}
		// Remove braces and split by comma
		v = strings.Trim(v, "{}")
		if v == "" {
			*t = Tags{}
			return nil
		}
		*t = Tags(strings.Split(v, ","))
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Tags", value)
	}
}

// Value implements the driver.Valuer interface for Tags
func (t Tags) Value() (driver.Value, error) {
	if len(t) == 0 {
		return pq.Array([]string{}), nil
	}
	return pq.Array([]string(t)), nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface
func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (v ValidationErrors) Error() string {
	var messages []string
	for _, err := range v {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// HasErrors returns true if there are validation errors
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// Add adds a validation error
func (v *ValidationErrors) Add(field, message string) {
	*v = append(*v, ValidationError{Field: field, Message: message})
}