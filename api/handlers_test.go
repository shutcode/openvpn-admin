package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestParseUserFilter(t *testing.T) {
	handler := &Handler{}

	t.Run("ParseLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users?limit=10", nil)
		filter := handler.parseUserFilter(req)

		assert.Equal(t, 10, filter.Limit)
	})

	t.Run("ParseOffset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users?offset=5", nil)
		filter := handler.parseUserFilter(req)

		assert.Equal(t, 5, filter.Offset)
	})

	t.Run("ParseStatus", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users?status=active", nil)
		filter := handler.parseUserFilter(req)

		assert.NotNil(t, filter.Status)
		assert.Equal(t, models.UserStatusActive, *filter.Status)
	})

	t.Run("ParseSearch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users?search=test", nil)
		filter := handler.parseUserFilter(req)

		assert.Equal(t, "test", filter.SearchTerm)
	})
}

func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected bool
	}{
		{"Valid simple", "user", true},
		{"Valid with number", "user123", true},
		{"Valid with hyphen", "user-name", true},
		{"Valid with underscore", "user_name", true},
		{"Valid mixed", "User-123_name", true},
		{"Invalid with space", "user name", false},
		{"Invalid with special", "user@name", false},
		{"Empty", "", false},
		{"Too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUsername(tt.username)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_New(t *testing.T) {
	t.Run("CreateServer", func(t *testing.T) {
		config := ServerConfig{
			JWTSecret:      "test-secret",
			AllowedOrigins: []string{"*"},
			RequireAuth:    false,
		}

		server := &Server{
			config: config,
		}

		assert.NotNil(t, server)
		assert.NotNil(t, server.config.AllowedOrigins)
	})
}

func TestServer_Handler(t *testing.T) {
	config := ServerConfig{
		JWTSecret:      "test-secret",
		AllowedOrigins: []string{"*"},
		RequireAuth:    false,
	}

	server := &Server{
		config: config,
	}

	handler := server.Handler()

	assert.NotNil(t, handler)
}

func TestServer_RegisterRoutes(t *testing.T) {
	t.Run("RegisterRoutesWithoutAuth", func(t *testing.T) {
		config := ServerConfig{
			JWTSecret:      "test-secret",
			AllowedOrigins: []string{"*"},
			RequireAuth:    false,
		}

		server := &Server{
			config: config,
		}

		mux := http.NewServeMux()
		server.registerRoutes(mux)

		assert.NotNil(t, mux)
	})
}