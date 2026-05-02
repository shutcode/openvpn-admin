package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the database connection with our custom methods
type DB struct {
	*sql.DB
	Path string
}

// Open opens a database connection with migrations applied
func Open(path string) (*DB, error) {
	// Ensure parent directory exists
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open database with foreign keys enabled
	connStr := fmt.Sprintf("%s?_fk=1&_journal_mode=WAL", path)
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		DB:   sqlDB,
		Path: path,
	}

	// Run migrations
	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Migrate runs all migrations from the embedded filesystem
func (db *DB) Migrate() error {
	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	// Create migrations table if not exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(file.Name(), "%d_", &version); err != nil {
			continue // Skip files that don't match the naming pattern
		}

		// Check if already applied
		var exists int
		if err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", version).Scan(&exists); err == nil {
			continue // Already applied
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + file.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file.Name(), err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file.Name(), err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", file.Name(), err)
		}
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.DB.BeginTx(ctx, nil)
}

// Register custom SQLite functions
func RegisterSQLiteFunctions() {
	// Custom functions can be registered here if needed
	// For now, we use application-generated UUIDs
	_ = sqlite.RegisterDeterministicScalarFunction // Avoid unused import
}
