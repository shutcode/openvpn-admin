package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shutcode/openvpn-admin/internal/db"
	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuditRepoTest(t *testing.T) (*auditRepository, func()) {
	tempDir, err := os.MkdirTemp("", "openvpn-audit-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)

	repo := NewAuditRepository(database.DB)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tempDir)
	}

	return repo.(*auditRepository), cleanup
}

func TestAuditRepository_Create(t *testing.T) {
	repo, cleanup := setupAuditRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CreateAuditLog", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		log := &models.AuditLog{
			Action:    models.AuditActionUserCreate,
			UserID:    &userID,
			Username:  "testuser",
			ActorID:   "admin",
			ActorType: "user",
			IPAddress: "192.168.1.100",
			UserAgent: "curl/7.68.0",
			Success:   true,
			Details:   `{"message": "User created"}`,
		}

		err := repo.Create(ctx, log)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, log.ID)
		assert.NotZero(t, log.Timestamp)
	})

	t.Run("CreateGeneratesID", func(t *testing.T) {
		log := &models.AuditLog{
			Action:    models.AuditActionLogin,
			ActorID:   "system",
			ActorType: "system",
			Success:   true,
		}

		err := repo.Create(ctx, log)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, log.ID)
	})
}

func TestAuditRepository_GetByID(t *testing.T) {
	repo, cleanup := setupAuditRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetExistingLog", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		log := &models.AuditLog{
			Action:    models.AuditActionUserCreate,
			UserID:    &userID,
			Username:  "testuser",
			ActorID:   "admin",
			ActorType: "user",
			Success:   true,
		}
		err := repo.Create(ctx, log)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, log.ID)

		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, log.Action, retrieved.Action)
		assert.Equal(t, log.Username, retrieved.Username)
	})

	t.Run("GetNonExistentLog", func(t *testing.T) {
		retrieved, err := repo.GetByID(ctx, uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestAuditRepository_List(t *testing.T) {
	repo, cleanup := setupAuditRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple audit logs
	for i := 0; i < 5; i++ {
		log := &models.AuditLog{
			Action:    models.AuditActionUserCreate,
			Username:  "user" + string(rune('a'+i)),
			ActorID:   "admin",
			ActorType: "user",
			Success:   true,
		}
		err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	t.Run("ListAll", func(t *testing.T) {
		logs, err := repo.List(ctx, models.AuditLogFilter{})

		assert.NoError(t, err)
		assert.Len(t, logs, 5)
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		logs, err := repo.List(ctx, models.AuditLogFilter{Limit: 2})

		assert.NoError(t, err)
		assert.Len(t, logs, 2)
	})

	t.Run("ListWithActionFilter", func(t *testing.T) {
		action := models.AuditActionUserCreate
		logs, err := repo.List(ctx, models.AuditLogFilter{Action: &action})

		assert.NoError(t, err)
		assert.Len(t, logs, 5)
	})

	t.Run("ListWithTimeFilter", func(t *testing.T) {
		// Use a time range that should include all logs
		// Start from a time before the first log was created
		// and end at a time far in the future
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now().Add(24 * time.Hour)
		logs, err := repo.List(ctx, models.AuditLogFilter{
			StartTime: &startTime,
			EndTime:   &endTime,
		})

		assert.NoError(t, err)
		assert.Len(t, logs, 5)
	})

	t.Run("ListWithUserIDFilter", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		log := &models.AuditLog{
			Action:    models.AuditActionUserUpdate,
			UserID:    &userID,
			ActorID:   "admin",
			ActorType: "user",
			Success:   true,
		}
		err := repo.Create(ctx, log)
		require.NoError(t, err)

		logs, err := repo.List(ctx, models.AuditLogFilter{UserID: &userID})

		assert.NoError(t, err)
		assert.Len(t, logs, 1)
	})
}

func TestAuditRepository_Count(t *testing.T) {
	repo, cleanup := setupAuditRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create audit logs
	for i := 0; i < 3; i++ {
		log := &models.AuditLog{
			Action:    models.AuditActionUserCreate,
			Username:  "user" + string(rune('a'+i)),
			ActorID:   "admin",
			ActorType: "user",
			Success:   true,
		}
		err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	t.Run("CountAll", func(t *testing.T) {
		count, err := repo.Count(ctx, models.AuditLogFilter{})

		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("CountWithActionFilter", func(t *testing.T) {
		action := models.AuditActionUserUpdate
		count, err := repo.Count(ctx, models.AuditLogFilter{Action: &action})

		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}