package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/db"
	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserServiceTest(t *testing.T) (*UserService, func()) {
	tempDir, err := os.MkdirTemp("", "openvpn-service-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(database.DB)
	auditRepo := repository.NewAuditRepository(database.DB)
	configRepo := repository.NewConfigRepository(database.DB)
	configCache := NewConfigCacheManager(tempDir, configRepo)

	config := CertificateWorkerConfig{
		EasyRsaPath: "/etc/openvpn/easy-rsa",
		OpenVPNPath: "/etc/openvpn",
		ClientsDir:  tempDir,
		WorkerCount: 1,
		QueueSize:   10,
	}
	certWorker := NewCertificateWorker(config, userRepo, configCache)

	userService := NewUserService(userRepo, auditRepo, certWorker, configCache)

	cleanup := func() {
		certWorker.Stop()
		database.Close()
		os.RemoveAll(tempDir)
	}

	return userService, cleanup
}

func TestNewUserService(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	assert.NotNil(t, service)
	assert.NotNil(t, service.userRepo)
	assert.NotNil(t, service.auditRepo)
	assert.NotNil(t, service.certWorker)
	assert.NotNil(t, service.configCache)
}

func TestUserService_CreateUser(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		req := models.CreateUserRequest{
			Name:  "newuser",
			Email: "newuser@example.com",
		}

		user, err := service.CreateUser(ctx, req, "admin", "user")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "newuser", user.Name)
		assert.Equal(t, "newuser@example.com", user.Email)
		assert.Equal(t, models.UserStatusInactive, user.Status)
	})

	t.Run("CreateUserDuplicateName", func(t *testing.T) {
		req := models.CreateUserRequest{
			Name:  "duplicate",
			Email: "duplicate@example.com",
		}

		// First create should succeed
		_, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)

		// Second create with same name should fail
		_, err = service.CreateUser(ctx, req, "admin", "user")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestUserService_GetUser(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetExistingUser", func(t *testing.T) {
		// Create user first
		req := models.CreateUserRequest{
			Name:  "gettest",
			Email: "gettest@example.com",
		}
		created, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)

		// Get user
		user, err := service.GetUser(ctx, created.ID)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, created.ID, user.ID)
	})

	t.Run("GetNonExistentUser", func(t *testing.T) {
		user, err := service.GetUser(ctx, uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.Nil(t, user)
	})
}

func TestUserService_GetUserByName(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetByNameExisting", func(t *testing.T) {
		req := models.CreateUserRequest{
			Name:  "nametest",
			Email: "nametest@example.com",
		}
		_, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)

		user, err := service.GetUserByName(ctx, "nametest")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "nametest", user.Name)
	})

	t.Run("GetByNameNonExistent", func(t *testing.T) {
		user, err := service.GetUserByName(ctx, "nonexistent")

		assert.NoError(t, err)
		assert.Nil(t, user)
	})
}

func TestUserService_ListUsers(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 3; i++ {
		req := models.CreateUserRequest{
			Name:  "listuser" + string(rune('a'+i)),
			Email: "listuser" + string(rune('a'+i)) + "@example.com",
		}
		_, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)
	}

	t.Run("ListAllUsers", func(t *testing.T) {
		result, err := service.ListUsers(ctx, repository.UserFilter{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Users, 3)
		assert.Equal(t, 3, result.Total)
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		result, err := service.ListUsers(ctx, repository.UserFilter{Limit: 2})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Users, 2)
	})

	t.Run("ListWithStatusFilter", func(t *testing.T) {
		status := models.UserStatusActive
		result, err := service.ListUsers(ctx, repository.UserFilter{Status: &status})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Initially all users are inactive until cert is generated
	})
}

func TestUserService_DeleteUser(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("DeleteExistingUser", func(t *testing.T) {
		// Create user
		req := models.CreateUserRequest{
			Name:  "deletetest",
			Email: "deletetest@example.com",
		}
		created, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)

		// Delete user
		err = service.DeleteUser(ctx, created.ID, "admin", "user")

		assert.NoError(t, err)

		// Verify user is revoked
		user, err := service.GetUser(ctx, created.ID)
		assert.NoError(t, err)
		assert.Equal(t, models.UserStatusRevoked, user.Status)
	})

	t.Run("DeleteNonExistentUser", func(t *testing.T) {
		err := service.DeleteUser(ctx, uuid.Must(uuid.NewV7()), "admin", "user")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserService_GetUserConfig(t *testing.T) {
	service, cleanup := setupUserServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetConfigNoCache", func(t *testing.T) {
		// Create user
		req := models.CreateUserRequest{
			Name:  "configtest",
			Email: "configtest@example.com",
		}
		created, err := service.CreateUser(ctx, req, "admin", "user")
		require.NoError(t, err)

		// Get config (will fail to read from file, which is expected)
		_, err = service.GetUserConfig(ctx, created.ID)

		// Expected to fail because config file doesn't exist
		assert.Error(t, err)
	})
}