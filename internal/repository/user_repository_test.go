package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/db"
	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserRepoTest(t *testing.T) (*userRepository, func()) {
	tempDir, err := os.MkdirTemp("", "openvpn-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)

	repo := NewUserRepository(database.DB)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tempDir)
	}

	return repo.(*userRepository), cleanup
}

func TestUserRepository_Create(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		user := &models.User{
			ID:     uuid.Must(uuid.NewV7()),
			Name:   "testuser1",
			Email:  "test1@example.com",
			Status: models.UserStatusActive,
		}

		err := repo.Create(ctx, user)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.NotZero(t, user.CreatedAt)
		assert.NotZero(t, user.UpdatedAt)
	})

	t.Run("CreateUserGeneratesID", func(t *testing.T) {
		user := &models.User{
			Name:   "testuser2",
			Email:  "test2@example.com",
			Status: models.UserStatusActive,
		}

		err := repo.Create(ctx, user)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, user.ID)
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetExistingUser", func(t *testing.T) {
		user := &models.User{
			Name:   "gettest",
			Email:  "gettest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, user.ID)

		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, user.Name, retrieved.Name)
		assert.Equal(t, user.Email, retrieved.Email)
	})

	t.Run("GetNonExistentUser", func(t *testing.T) {
		retrieved, err := repo.GetByID(ctx, uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUserRepository_GetByName(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetByNameExisting", func(t *testing.T) {
		user := &models.User{
			Name:   "nametest",
			Email:  "nametest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetByName(ctx, "nametest")

		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, user.ID, retrieved.ID)
	})

	t.Run("GetByNameNonExistent", func(t *testing.T) {
		retrieved, err := repo.GetByName(ctx, "nonexistent")

		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUserRepository_Update(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("UpdateUser", func(t *testing.T) {
		user := &models.User{
			Name:   "updatetest",
			Email:  "updatetest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		user.Email = "updated@example.com"
		err = repo.Update(ctx, user)

		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, "updated@example.com", retrieved.Email)
	})
}

func TestUserRepository_UpdateStatus(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("UpdateStatus", func(t *testing.T) {
		user := &models.User{
			Name:   "statustest",
			Email:  "statustest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, user.ID, models.UserStatusRevoked)
		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, models.UserStatusRevoked, retrieved.Status)
	})
}

func TestUserRepository_UpdateConnectionInfo(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("UpdateConnectionInfo", func(t *testing.T) {
		user := &models.User{
			Name:   "conntest",
			Email:  "conntest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		err = repo.UpdateConnectionInfo(ctx, user.ID, "10.8.0.5", "192.168.1.50")
		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, "10.8.0.5", retrieved.VirtualIP)
		assert.Equal(t, "192.168.1.50", retrieved.RealIP)
		assert.NotNil(t, retrieved.LastConnected)
	})
}

func TestUserRepository_Delete(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("DeleteUser", func(t *testing.T) {
		user := &models.User{
			Name:   "deletetest",
			Email:  "deletetest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		err = repo.Delete(ctx, user.ID)
		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUserRepository_List(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 5; i++ {
		user := &models.User{
			Name:   "listuser" + string(rune('a'+i)),
			Email:  "listuser" + string(rune('a'+i)) + "@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)
	}

	t.Run("ListAll", func(t *testing.T) {
		users, err := repo.List(ctx, UserFilter{})

		assert.NoError(t, err)
		assert.Len(t, users, 5)
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		users, err := repo.List(ctx, UserFilter{Limit: 2})

		assert.NoError(t, err)
		assert.Len(t, users, 2)
	})

	t.Run("ListWithStatusFilter", func(t *testing.T) {
		status := models.UserStatusInactive
		users, err := repo.List(ctx, UserFilter{Status: &status})

		assert.NoError(t, err)
		assert.Len(t, users, 0)
	})

	t.Run("ListWithSearchTerm", func(t *testing.T) {
		users, err := repo.List(ctx, UserFilter{SearchTerm: "listuser"})

		assert.NoError(t, err)
		assert.Len(t, users, 5)
	})
}

func TestUserRepository_Count(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create users
	for i := 0; i < 3; i++ {
		user := &models.User{
			Name:   "countuser" + string(rune('a'+i)),
			Email:  "countuser" + string(rune('a'+i)) + "@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)
	}

	t.Run("CountAll", func(t *testing.T) {
		count, err := repo.Count(ctx, UserFilter{})

		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("CountWithStatusFilter", func(t *testing.T) {
		status := models.UserStatusInactive
		count, err := repo.Count(ctx, UserFilter{Status: &status})

		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestUserRepository_ExistsByName(t *testing.T) {
	repo, cleanup := setupUserRepoTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Exists", func(t *testing.T) {
		user := &models.User{
			Name:   "existstest",
			Email:  "existstest@example.com",
			Status: models.UserStatusActive,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		exists, err := repo.ExistsByName(ctx, "existstest")

		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("DoesNotExist", func(t *testing.T) {
		exists, err := repo.ExistsByName(ctx, "nonexistent")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}