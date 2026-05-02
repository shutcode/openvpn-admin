package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims represents the JWT claims for our application
type Claims struct {
	jwt.RegisteredClaims
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	TokenType TokenType `json:"token_type"`
}

// JWTConfig holds configuration for JWT operations
type JWTConfig struct {
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
}

// DefaultJWTConfig returns a default configuration
func DefaultJWTConfig(secret string) JWTConfig {
	return JWTConfig{
		AccessSecret:  []byte(secret),
		RefreshSecret: []byte(secret + "_refresh"),
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		Issuer:        "openvpn-mgmt",
	}
}

// JWTManager handles JWT operations
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager creates a new JWTManager
func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{config: config}
}

// GenerateTokenPair generates both access and refresh tokens
func (m *JWTManager) GenerateTokenPair(userID uuid.UUID, username, role string) (*TokenPair, error) {
	accessToken, err := m.generateToken(userID, username, role, TokenTypeAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := m.generateToken(userID, username, role, TokenTypeRefresh)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateToken creates a JWT token with the specified claims
func (m *JWTManager) generateToken(userID uuid.UUID, username, role string, tokenType TokenType) (string, error) {
	var secret []byte
	var ttl time.Duration

	switch tokenType {
	case TokenTypeAccess:
		secret = m.config.AccessSecret
		ttl = m.config.AccessTTL
	case TokenTypeRefresh:
		secret = m.config.RefreshSecret
		ttl = m.config.RefreshTTL
	default:
		return "", fmt.Errorf("invalid token type: %s", tokenType)
	}

	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
		},
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateAccessToken validates an access token and returns the claims
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, TokenTypeAccess)
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (m *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return m.validateToken(tokenString, TokenTypeRefresh)
}

// validateToken validates a token and returns the claims
func (m *JWTManager) validateToken(tokenString string, tokenType TokenType) (*Claims, error) {
	var secret []byte

	switch tokenType {
	case TokenTypeAccess:
		secret = m.config.AccessSecret
	case TokenTypeRefresh:
		secret = m.config.RefreshSecret
	default:
		return nil, fmt.Errorf("invalid token type: %s", tokenType)
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	// Verify token type matches
	if claims.TokenType != tokenType {
		return nil, fmt.Errorf("token type mismatch: expected %s, got %s", tokenType, claims.TokenType)
	}

	return claims, nil
}

// RefreshTokenPair generates a new token pair from a valid refresh token
func (m *JWTManager) RefreshTokenPair(refreshToken string) (*TokenPair, error) {
	claims, err := m.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	return m.GenerateTokenPair(claims.UserID, claims.Username, claims.Role)
}

// TokenPair represents a pair of access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// contextKey is the type for context keys
type contextKey int

const (
	// ClaimsContextKey is the context key for JWT claims
	ClaimsContextKey contextKey = iota
)

// ContextWithClaims adds claims to the context
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}

// ClaimsFromContext extracts claims from the context
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	return claims, ok
}
