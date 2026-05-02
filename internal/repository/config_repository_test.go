package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConfigRepoTest(t *testing.T) (*configRepository, func()) {
	tempDir, err := os.MkdirTemp("", "openvpn-config-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)

	repo := NewConfigRepository(database.DB)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tempDir)
	}

	return repo.(*configRepository), cleanup
}

func TestConfigRepository_Save(t *testing.T) {
	repo, cleanup := setupConfigRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SaveNewConfig", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("client config content")

		err := repo.Save(ctx, userID, configData)

		assert.NoError(t, err)
	})

	t.Run("UpdateExistingConfig", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		initialData := []byte("initial config")
		updatedData := []byte("updated config")

		err := repo.Save(ctx, userID, initialData)
		require.NoError(t, err)

		err = repo.Save(ctx, userID, updatedData)
		assert.NoError(t, err)

		// Verify updated data
		data, err := repo.Get(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, updatedData, data)
	})
}

func TestConfigRepository_Get(t *testing.T) {
	repo, cleanup := setupConfigRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetExistingConfig", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		err := repo.Save(ctx, userID, configData)
		require.NoError(t, err)

		data, err := repo.Get(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, configData, data)
	})

	t.Run("GetNonExistentConfig", func(t *testing.T) {
		data, err := repo.Get(ctx, uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.Nil(t, data)
	})
}

func TestConfigRepository_GetByUserID(t *testing.T) {
	repo, cleanup := setupConfigRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetByUserIDExisting", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		err := repo.Save(ctx, userID, configData)
		require.NoError(t, err)

		cache, err := repo.GetByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, userID, cache.UserID)
		assert.Equal(t, configData, cache.ConfigData)
	})

	t.Run("GetByUserIDNonExistent", func(t *testing.T) {
		cache, err := repo.GetByUserID(ctx, uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.Nil(t, cache)
	})
}

func TestConfigRepository_Delete(t *testing.T) {
	repo, cleanup := setupConfigRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("DeleteConfig", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		err := repo.Save(ctx, userID, configData)
		require.NoError(t, err)

		err = repo.Delete(ctx, userID)
		assert.NoError(t, err)

		// Verify deleted
		data, err := repo.Get(ctx, userID)
		assert.NoError(t, err)
		assert.Nil(t, data)
	})
}

func TestConfigRepository_DeleteByID(t *testing.T) {
	repo, cleanup := setupConfigRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("DeleteByID", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		err := repo.Save(ctx, userID, configData)
		require.NoError(t, err)

		// Get the config first to get its ID
		cache, err := repo.GetByUserID(ctx, userID)
		require.NoError(t, err)

		err = repo.DeleteByID(ctx, cache.ID)
		assert.NoError(t, err)

		// Verify deleted
		data, err := repo.Get(ctx, userID)
		assert.NoError(t, err)
		assert.Nil(t, data)
	})
}

func TestConfigCacheModel(t *testing.T) {
	t.Run("CreateAndRetrieve", func(t *testing.T) {
		repo, cleanup := setupConfigRepoTest(t)
		defer cleanup()

		ctx := context.Background()
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("client ovpn config")

		err := repo.Save(ctx, userID, configData)
		require.NoError(t, err)

		cache, err := repo.GetByUserID(ctx, userID)
		require.NoError(t, err)

		// Verify model fields
		assert.Equal(t, userID, cache.UserID)
		assert.Equal(t, configData, cache.ConfigData)
		assert.NotZero(t, cache.CreatedAt)
		assert.NotZero(t, cache.UpdatedAt)
	})
}