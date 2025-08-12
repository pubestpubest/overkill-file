package migrations

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestMigrations(t *testing.T) {
	// Skip if no test database URL is provided
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping migration tests")
	}

	// Connect to test database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	// Clean up any existing schema
	_, err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	if err != nil {
		t.Fatalf("Failed to clean test database: %v", err)
	}

	// Create migrator and run migrations
	migrator := NewMigrator(db)
	err = migrator.RunMigrations()
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify schema_migrations table exists and has correct version
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema version: %v", err)
	}

	if version != 1 {
		t.Errorf("Expected schema version 1, got %d", version)
	}

	// Verify all tables exist
	tables := []string{"users", "files", "shares", "activities"}
	for _, table := range tables {
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("Table %s does not exist", table)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_files_user_id",
		"idx_files_tags", 
		"idx_shares_token",
		"idx_shares_expires",
		"idx_activities_user_id",
	}
	for _, index := range indexes {
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)`, index).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check index %s: %v", index, err)
		}
		if !exists {
			t.Errorf("Index %s does not exist", index)
		}
	}

	// Test running migrations again (should be idempotent)
	err = migrator.RunMigrations()
	if err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	// Verify version is still 1
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema version after second run: %v", err)
	}

	if version != 1 {
		t.Errorf("Expected schema version 1 after second run, got %d", version)
	}
}