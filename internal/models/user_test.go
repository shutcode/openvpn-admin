package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   UserStatus
		expected string
	}{
		{"Active", UserStatusActive, "active"},
		{"Inactive", UserStatusInactive, "inactive"},
		{"Revoked", UserStatusRevoked, "revoked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestUserStruct(t *testing.T) {
	t.Run("CreateUser", func(t *testing.T) {
		id := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()

		user := &User{
			ID:       id,
			Name:     "testuser",
			Email:    "test@example.com",
			Status:   UserStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}

		assert.Equal(t, id, user.ID)
		assert.Equal(t, "testuser", user.Name)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, UserStatusActive, user.Status)
		assert.NotZero(t, user.CreatedAt)
		assert.NotZero(t, user.UpdatedAt)
	})

	t.Run("UserWithConnectionInfo", func(t *testing.T) {
		lastConnected := time.Now().UTC()
		certExpiry := time.Now().Add(365 * 24 * time.Hour).UTC()

		user := &User{
			ID:            uuid.Must(uuid.NewV7()),
			Name:          "testuser",
			Email:         "test@example.com",
			Status:        UserStatusActive,
			LastConnected: &lastConnected,
			VirtualIP:     "10.8.0.2",
			RealIP:        "192.168.1.100",
			CertSerial:    "ABC123",
			CertExpiry:    &certExpiry,
		}

		assert.NotNil(t, user.LastConnected)
		assert.Equal(t, "10.8.0.2", user.VirtualIP)
		assert.Equal(t, "192.168.1.100", user.RealIP)
		assert.Equal(t, "ABC123", user.CertSerial)
		assert.NotNil(t, user.CertExpiry)
	})
}

func TestCreateUserRequest(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		req := CreateUserRequest{
			Name:  "testuser",
			Email: "test@example.com",
		}

		assert.Equal(t, "testuser", req.Name)
		assert.Equal(t, "test@example.com", req.Email)
	})

	t.Run("EmptyEmail", func(t *testing.T) {
		req := CreateUserRequest{
			Name: "testuser",
		}

		assert.Equal(t, "testuser", req.Name)
		assert.Empty(t, req.Email)
	})
}

func TestUpdateUserRequest(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		req := UpdateUserRequest{
			Email:  "updated@example.com",
			Status: UserStatusInactive,
		}

		assert.Equal(t, "updated@example.com", req.Email)
		assert.Equal(t, UserStatusInactive, req.Status)
	})
}

func TestUserListResponse(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		resp := UserListResponse{
			Users: []User{},
			Total: 0,
		}

		assert.Empty(t, resp.Users)
		assert.Equal(t, 0, resp.Total)
	})

	t.Run("WithUsers", func(t *testing.T) {
		users := []User{
			{ID: uuid.Must(uuid.NewV7()), Name: "user1"},
			{ID: uuid.Must(uuid.NewV7()), Name: "user2"},
		}

		resp := UserListResponse{
			Users: users,
			Total: 2,
		}

		assert.Len(t, resp.Users, 2)
		assert.Equal(t, 2, resp.Total)
	})
}

func TestConnectedUserInfo(t *testing.T) {
	t.Run("FullInfo", func(t *testing.T) {
		connectedSince := time.Now().UTC()
		info := ConnectedUserInfo{
			Name:           "testuser",
			RealIP:         "192.168.1.100",
			VirtualIP:      "10.8.0.2",
			ConnectedSince: connectedSince,
			BytesReceived:  1024000,
			BytesSent:      512000,
		}

		assert.Equal(t, "testuser", info.Name)
		assert.Equal(t, "10.8.0.2", info.VirtualIP)
		assert.Equal(t, int64(1024000), info.BytesReceived)
		assert.Equal(t, int64(512000), info.BytesSent)
	})
}