package migrations

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrator handles database migrations
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// RunMigrations executes all pending migrations
func (m *Migrator) RunMigrations() error {
	// Ensure schema_migrations table exists
	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current schema version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Load all migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Filter and sort pending migrations
	var pendingMigrations []Migration
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	sort.Slice(pendingMigrations, func(i, j int) bool {
		return pendingMigrations[i].Version < pendingMigrations[j].Version
	})

	// Execute pending migrations
	for _, migration := range pendingMigrations {
		log.Printf("Running migration %d: %s", migration.Version, migration.Name)
		if err := m.executeMigration(migration); err != nil {
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}
		log.Printf("Successfully applied migration %d", migration.Version)
	}

	if len(pendingMigrations) == 0 {
		log.Println("No pending migrations to run")
	} else {
		log.Printf("Successfully applied %d migrations", len(pendingMigrations))
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)`
	_, err := m.db.Exec(query)
	return err
}

// getCurrentVersion returns the current schema version
func (m *Migrator) getCurrentVersion() (int, error) {
	var version int
	query := "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"
	err := m.db.QueryRow(query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// loadMigrations loads all migration files from the migrations directory
func (m *Migrator) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	// Walk through the migrations directory
	err := filepath.WalkDir("migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-SQL files
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Skip the migrate.go file
		if strings.HasSuffix(path, ".go") {
			return nil
		}

		// Parse migration version and name from filename
		filename := filepath.Base(path)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid migration filename format: %s", filename)
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid migration version in filename %s: %w", filename, err)
		}

		name := strings.TrimSuffix(parts[1], ".sql")

		// Read migration SQL content from file system
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return migrations, nil
}

// executeMigration executes a single migration within a transaction
func (m *Migrator) executeMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the migration SQL
	_, err = tx.Exec(migration.SQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record the migration as applied
	_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", migration.Version)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// GetCurrentVersion returns the current schema version (public method)
func (m *Migrator) GetCurrentVersion() (int, error) {
	return m.getCurrentVersion()
}