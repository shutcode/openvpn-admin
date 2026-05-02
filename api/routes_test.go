package api

import (
	"testing"

	"github.com/shutcode/openvpn-admin/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJWTManager_Integration(t *testing.T) {
	config := ServerConfig{
		JWTSecret:      "test-secret-key",
		AllowedOrigins: []string{"*"},
		RequireAuth:    true,
	}

	jwtConfig := auth.DefaultJWTConfig(config.JWTSecret)
	jwtManager := auth.NewJWTManager(jwtConfig)

	t.Run("GenerateAndValidateToken", func(t *testing.T) {
		userID := uuid.New()
		username := "testuser"
		role := "admin"

		pair, err := jwtManager.GenerateTokenPair(userID, username, role)
		assert.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)

		claims, err := jwtManager.ValidateAccessToken(pair.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, username, claims.Username)
		assert.Equal(t, role, claims.Role)
	})
}