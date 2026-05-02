package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJWTConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultJWTConfig("mysecret")

		assert.NotEmpty(t, config.AccessSecret)
		assert.NotEmpty(t, config.RefreshSecret)
		assert.NotZero(t, config.AccessTTL)
		assert.NotZero(t, config.RefreshTTL)
		assert.Equal(t, "openvpn-mgmt", config.Issuer)
	})
}

func TestJWTManager_GenerateTokenPair(t *testing.T) {
	config := DefaultJWTConfig("test-secret-key")
	manager := NewJWTManager(config)

	t.Run("GenerateTokenPair", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())

		pair, err := manager.GenerateTokenPair(userID, "testuser", "admin")

		assert.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)
	})
}

func TestJWTManager_ValidateAccessToken(t *testing.T) {
	config := DefaultJWTConfig("test-secret-key")
	manager := NewJWTManager(config)

	t.Run("ValidAccessToken", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		pair, err := manager.GenerateTokenPair(userID, "testuser", "admin")
		require.NoError(t, err)

		claims, err := manager.ValidateAccessToken(pair.AccessToken)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "admin", claims.Role)
		assert.Equal(t, TokenTypeAccess, claims.TokenType)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		claims, err := manager.ValidateAccessToken("invalid-token")

		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("WrongTokenType", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		pair, err := manager.GenerateTokenPair(userID, "testuser", "admin")
		require.NoError(t, err)

		// Try to validate refresh token as access token
		claims, err := manager.ValidateAccessToken(pair.RefreshToken)

		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestJWTManager_ValidateRefreshToken(t *testing.T) {
	config := DefaultJWTConfig("test-secret-key")
	manager := NewJWTManager(config)

	t.Run("ValidRefreshToken", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		pair, err := manager.GenerateTokenPair(userID, "testuser", "admin")
		require.NoError(t, err)

		claims, err := manager.ValidateRefreshToken(pair.RefreshToken)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, TokenTypeRefresh, claims.TokenType)
	})
}

func TestJWTManager_RefreshTokenPair(t *testing.T) {
	config := DefaultJWTConfig("test-secret-key")
	manager := NewJWTManager(config)

	t.Run("RefreshToken", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		pair, err := manager.GenerateTokenPair(userID, "testuser", "admin")
		require.NoError(t, err)

		newPair, err := manager.RefreshTokenPair(pair.RefreshToken)

		assert.NoError(t, err)
		assert.NotEmpty(t, newPair.AccessToken)
		assert.NotEmpty(t, newPair.RefreshToken)
		// The refreshed token should be different or at least valid
		claims, err := manager.ValidateAccessToken(newPair.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
	})

	t.Run("InvalidRefreshToken", func(t *testing.T) {
		newPair, err := manager.RefreshTokenPair("invalid-token")

		assert.Error(t, err)
		assert.Nil(t, newPair)
	})
}

func TestJWTManager_ContextWithClaims(t *testing.T) {
	t.Run("AddAndRetrieveClaims", func(t *testing.T) {
		ctx := NewTestContext()
		userID := uuid.Must(uuid.NewV7())
		claims := &Claims{
			UserID:    userID,
			Username:  "testuser",
			Role:      "admin",
			TokenType: TokenTypeAccess,
		}

		ctxWithClaims := ContextWithClaims(ctx, claims)

		retrievedClaims, ok := ClaimsFromContext(ctxWithClaims)
		assert.True(t, ok)
		assert.Equal(t, userID, retrievedClaims.UserID)
		assert.Equal(t, "testuser", retrievedClaims.Username)
	})

	t.Run("NoClaimsInContext", func(t *testing.T) {
		ctx := NewTestContext()

		_, ok := ClaimsFromContext(ctx)
		assert.False(t, ok)
	})
}

func TestTokenType(t *testing.T) {
	t.Run("TokenTypes", func(t *testing.T) {
		assert.Equal(t, "access", string(TokenTypeAccess))
		assert.Equal(t, "refresh", string(TokenTypeRefresh))
	})
}

func TestClaims(t *testing.T) {
	t.Run("CreateClaims", func(t *testing.T) {
		claims := &Claims{
			UserID:    uuid.Must(uuid.NewV7()),
			Username:  "testuser",
			Role:      "admin",
			TokenType: TokenTypeAccess,
		}

		assert.NotZero(t, claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "admin", claims.Role)
	})
}

// NewTestContext creates a new context for testing
func NewTestContext() *TestCtx {
	return &TestCtx{}
}

type TestCtx struct{}

func (c *TestCtx) Value(key interface{}) interface{} {
	return nil
}

func (c *TestCtx) Deadline() (time.Time, bool) {
	return time.Now(), false
}

func (c *TestCtx) Done() <-chan struct{} {
	return nil
}

func (c *TestCtx) Err() error {
	return nil
}