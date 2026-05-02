package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shutcode/openvpn-admin/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestChain(t *testing.T) {
	t.Run("ChainMultipleMiddleware", func(t *testing.T) {
		callOrder := []string{}

		m1 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "m1")
				next.ServeHTTP(w, r)
			})
		}

		m2 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "m2")
				next.ServeHTTP(w, r)
			})
		}

		final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "final")
		})

		handler := Chain(m1, m2)(final)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, []string{"m1", "m2", "final"}, callOrder)
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("LogsRequest", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := LoggingMiddleware(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("RecoversFromPanic", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		handler := RecoveryMiddleware(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("NoPanic", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RecoveryMiddleware(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestCORSMiddleware(t *testing.T) {
	t.Run("WildcardAllowed", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := CORSMiddleware([]string{"*"})(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, "http://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("SpecificOriginAllowed", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := CORSMiddleware([]string{"http://trusted.com"})(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://trusted.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, "http://trusted.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("OriginNotAllowed", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := CORSMiddleware([]string{"http://trusted.com"})(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://untrusted.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("PreflightRequest", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := CORSMiddleware([]string{"*"})(next)

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})
}

func TestAuthMiddleware(t *testing.T) {
	jwtConfig := auth.DefaultJWTConfig("test-secret")
	jwtManager := auth.NewJWTManager(jwtConfig)

	t.Run("ValidToken", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			assert.True(t, ok)
			assert.NotNil(t, claims)
		})

		handler := AuthMiddleware(jwtManager)(next)

		// Generate valid token
		pair, err := jwtManager.GenerateTokenPair(uuid.New(), "testuser", "admin")
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("NoAuthHeader", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

		handler := AuthMiddleware(jwtManager)(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

		handler := AuthMiddleware(jwtManager)(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("WrongTokenType", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

		handler := AuthMiddleware(jwtManager)(next)

		// Use refresh token as access token
		pair, err := jwtManager.GenerateTokenPair(uuid.New(), "testuser", "admin")
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+pair.RefreshToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestRequireRole(t *testing.T) {
	t.Run("AdminHasAccess", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RequireRole("user")(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// Add claims to context
		ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{
			UserID:   uuid.New(),
			Username: "testuser",
			Role:     "admin",
		})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("CorrectRoleHasAccess", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RequireRole("user")(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{
			UserID:   uuid.New(),
			Username: "testuser",
			Role:     "user",
		})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("WrongRoleDenied", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

		handler := RequireRole("admin")(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{
			UserID:   uuid.New(),
			Username: "testuser",
			Role:     "user",
		})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("NoClaimsDenied", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})

		handler := RequireRole("admin")(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("WriteSuccess", func(t *testing.T) {
		rr := httptest.NewRecorder()

		writeJSON(rr, http.StatusOK, map[string]string{"message": "success"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var resp responseEnvelope
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "success", resp.Data.(map[string]interface{})["message"])
	})
}

func TestWriteError(t *testing.T) {
	t.Run("WriteError", func(t *testing.T) {
		rr := httptest.NewRecorder()

		writeError(rr, http.StatusBadRequest, "Bad request")

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var resp responseEnvelope
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Success)
		assert.NotNil(t, resp.Error)
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("CaptureStatusCode", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)

		assert.Equal(t, http.StatusCreated, rw.statusCode)
		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("WriteWritesHeader", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

		rw.Write([]byte("test"))

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}