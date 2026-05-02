package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAuditAction(t *testing.T) {
	tests := []struct {
		name     string
		action   AuditAction
		expected string
	}{
		{"UserCreate", AuditActionUserCreate, "user.create"},
		{"UserUpdate", AuditActionUserUpdate, "user.update"},
		{"UserDelete", AuditActionUserDelete, "user.delete"},
		{"UserRevoke", AuditActionUserRevoke, "user.revoke"},
		{"CertGenerate", AuditActionCertGenerate, "cert.generate"},
		{"CertRevoke", AuditActionCertRevoke, "cert.revoke"},
		{"ConfigExport", AuditActionConfigExport, "config.export"},
		{"Login", AuditActionLogin, "auth.login"},
		{"Logout", AuditActionLogout, "auth.logout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.action))
		})
	}
}

func TestAuditLog(t *testing.T) {
	t.Run("CreateAuditLog", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()

		log := &AuditLog{
			ID:        uuid.Must(uuid.NewV7()),
			Timestamp: now,
			Action:    AuditActionUserCreate,
			UserID:    &userID,
			Username:  "testuser",
			ActorID:   "admin",
			ActorType: "user",
			IPAddress: "192.168.1.100",
			UserAgent: "curl/7.68.0",
			Success:   true,
			Details:   `{"message": "User created successfully"}`,
		}

		assert.NotEqual(t, uuid.Nil, log.ID)
		assert.NotZero(t, log.Timestamp)
		assert.Equal(t, AuditActionUserCreate, log.Action)
		assert.NotNil(t, log.UserID)
		assert.Equal(t, "testuser", log.Username)
		assert.Equal(t, "admin", log.ActorID)
		assert.Equal(t, "user", log.ActorType)
		assert.Equal(t, "192.168.1.100", log.IPAddress)
		assert.True(t, log.Success)
	})

	t.Run("AuditLogWithoutUser", func(t *testing.T) {
		log := &AuditLog{
			ID:        uuid.Must(uuid.NewV7()),
			Timestamp: time.Now().UTC(),
			Action:    AuditActionLogin,
			ActorID:   "system",
			ActorType: "system",
			IPAddress: "127.0.0.1",
			Success:   true,
		}

		assert.Nil(t, log.UserID)
		assert.Empty(t, log.Username)
		assert.Equal(t, "system", log.ActorID)
		assert.Equal(t, "system", log.ActorType)
	})
}

func TestCreateAuditLogRequest(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())

		req := CreateAuditLogRequest{
			Action:    AuditActionUserCreate,
			UserID:    &userID,
			Username:  "testuser",
			ActorID:   "admin",
			ActorType: "user",
			IPAddress: "192.168.1.100",
			UserAgent: "curl/7.68.0",
			Success:   true,
			Details:   "User created",
		}

		assert.Equal(t, AuditActionUserCreate, req.Action)
		assert.NotNil(t, req.UserID)
		assert.Equal(t, "testuser", req.Username)
		assert.Equal(t, "admin", req.ActorID)
		assert.Equal(t, "user", req.ActorType)
		assert.True(t, req.Success)
	})
}

func TestAuditLogListResponse(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		resp := AuditLogListResponse{
			Logs:  []AuditLog{},
			Total: 0,
		}

		assert.Empty(t, resp.Logs)
		assert.Equal(t, 0, resp.Total)
	})
}

func TestAuditLogFilter(t *testing.T) {
	t.Run("CreateFilter", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		action := AuditActionUserCreate
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()

		filter := AuditLogFilter{
			Action:    &action,
			UserID:    &userID,
			StartTime: &startTime,
			EndTime:   &endTime,
			Limit:     50,
			Offset:    0,
		}

		assert.NotNil(t, filter.Action)
		assert.NotNil(t, filter.UserID)
		assert.NotNil(t, filter.StartTime)
		assert.NotNil(t, filter.EndTime)
		assert.Equal(t, 50, filter.Limit)
	})

	t.Run("EmptyFilter", func(t *testing.T) {
		filter := AuditLogFilter{}

		assert.Nil(t, filter.Action)
		assert.Nil(t, filter.UserID)
		assert.Empty(t, filter.ActorID)
		assert.Nil(t, filter.StartTime)
		assert.Nil(t, filter.EndTime)
		assert.Equal(t, 0, filter.Limit)
		assert.Equal(t, 0, filter.Offset)
	})
}