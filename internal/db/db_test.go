package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/google/uuid"
)

func TestDatabase(t *testing.T) {
	// Create temp directory for test DB
	tempDir, err := os.MkdirTemp("", "openvpn-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	t.Run("OpenAndMigrate", func(t *testing.T) {
		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Verify tables exist by running a simple query
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query tables: %v", err)
		}

		// Should have at least: users, audit_logs, config_cache, api_keys, certificates, schema_migrations
		if count < 6 {
			t.Errorf("Expected at least 6 tables, got %d", count)
		}
		t.Logf("Database has %d tables", count)
	})

	t.Run("UserRepository", func(t *testing.T) {
		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		repo := repository.NewUserRepository(db.DB)
		ctx := context.Background()

		// Test Create
		user := &models.User{
			ID:       uuid.Must(uuid.NewV7()),
			Name:     "testuser",
			Email:    "test@example.com",
			Status:   models.UserStatusActive,
		}

		err = repo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
		t.Logf("Created user: %s (%s)", user.Name, user.ID)

		// Test GetByID
		retrieved, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user by ID: %v", err)
		}
		if retrieved == nil {
			t.Fatal("Expected user, got nil")
		}
		if retrieved.Name != user.Name {
			t.Errorf("Expected name %s, got %s", user.Name, retrieved.Name)
		}

		// Test GetByName
		byName, err := repo.GetByName(ctx, user.Name)
		if err != nil {
			t.Fatalf("Failed to get user by name: %v", err)
		}
		if byName == nil {
			t.Fatal("Expected user, got nil")
		}

		// Test ExistsByName
		exists, err := repo.ExistsByName(ctx, user.Name)
		if err != nil {
			t.Fatalf("Failed to check user existence: %v", err)
		}
		if !exists {
			t.Error("Expected user to exist")
		}

		// Test UpdateStatus
		err = repo.UpdateStatus(ctx, user.ID, models.UserStatusInactive)
		if err != nil {
			t.Fatalf("Failed to update status: %v", err)
		}

		// Verify status change
		updated, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get updated user: %v", err)
		}
		if updated.Status != models.UserStatusInactive {
			t.Errorf("Expected status %s, got %s", models.UserStatusInactive, updated.Status)
		}

		// Test Count
		count, err := repo.Count(ctx, repository.UserFilter{})
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}

		// Test List
		users, err := repo.List(ctx, repository.UserFilter{})
		if err != nil {
			t.Fatalf("Failed to list users: %v", err)
		}
		if len(users) != 1 {
			t.Errorf("Expected 1 user, got %d", len(users))
		}

		t.Log("All UserRepository tests passed!")
	})
}
