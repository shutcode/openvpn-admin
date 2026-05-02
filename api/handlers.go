package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shutcode/openvpn-admin/internal/auth"
	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/shutcode/openvpn-admin/internal/service"
	"github.com/google/uuid"
)

// Handler holds all HTTP handlers
type Handler struct {
	userService *service.UserService
	jwtManager  *auth.JWTManager
	certWorker  *service.CertificateWorker
	config      HandlerConfig
}

// HandlerConfig holds handler configuration
type HandlerConfig struct {
	OpenVPNClientsDir string
	StatusFilePath    string
	OpenVPNPath       string
	EasyRSAPath       string
}

// DefaultHandlerConfig returns default configuration
func DefaultHandlerConfig() HandlerConfig {
	return HandlerConfig{
		OpenVPNClientsDir: "/etc/openvpn/clients",
		StatusFilePath:    "/var/log/openvpn/status.log",
		OpenVPNPath:       "/etc/openvpn",
		EasyRSAPath:       "/etc/openvpn/easy-rsa",
	}
}

// NewHandler creates a new Handler
func NewHandler(
	userService *service.UserService,
	jwtManager *auth.JWTManager,
	certWorker *service.CertificateWorker,
	config HandlerConfig,
) *Handler {
	return &Handler{
		userService: userService,
		jwtManager:  jwtManager,
		certWorker:  certWorker,
		config:      config,
	}
}

// ListUsersHandler handles GET /api/users
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for pagination and filtering
	filter := h.parseUserFilter(r)

	result, err := h.userService.ListUsers(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list users: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// parseUserFilter parses query parameters into a UserFilter
func (h *Handler) parseUserFilter(r *http.Request) repository.UserFilter {
	filter := repository.UserFilter{}

	// Parse pagination
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	// Parse status filter
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := models.UserStatus(statusStr)
		filter.Status = &status
	}

	// Parse search term
	filter.SearchTerm = r.URL.Query().Get("search")

	return filter
}

// CreateUserHandler handles POST /api/users
func (h *Handler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "User name is required")
		return
	}

	// Validate username format
	if !isValidUsername(req.Name) {
		writeError(w, http.StatusBadRequest, "Invalid username. Use alphanumeric characters, hyphens, and underscores only")
		return
	}

	// Get actor info from context (if available)
	actorID, actorType := h.getActorInfo(ctx)

	user, err := h.userService.CreateUser(ctx, req, actorID, actorType)
	if err != nil {
		// Check for specific errors
		if err.Error() == fmt.Sprintf("user with name '%s' already exists", req.Name) {
			writeError(w, http.StatusConflict, "User already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// GetUserHandler handles GET /api/users/:id
func (h *Handler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from URL
	userID, err := h.extractUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get user: %v", err))
		return
	}

	if user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DeleteUserHandler handles DELETE /api/users/:id
func (h *Handler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from URL
	userID, err := h.extractUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get actor info from context
	actorID, actorType := h.getActorInfo(ctx)

	if err := h.userService.DeleteUser(ctx, userID, actorID, actorType); err != nil {
		if err.Error() == "user not found" {
			writeError(w, http.StatusNotFound, "User not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete user: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetUserConfigHandler handles GET /api/users/:id/config
func (h *Handler) GetUserConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user ID from URL
	userID, err := h.extractUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get user first to find the username for the filename
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get user: %v", err))
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	// Get config data
	config, err := h.userService.GetUserConfig(ctx, userID)
	if err != nil {
		// Try to read directly from disk as fallback
		configPath := filepath.Join(h.config.OpenVPNClientsDir, user.Name+".ovpn")
		config, err = os.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "Configuration file not found")
				return
			}
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
			return
		}
	}

	// Set headers for file download
	w.Header().Set("Content-Type", "application/x-openvpn-profile")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.ovpn\"", user.Name))
	w.Header().Set("Content-Length", strconv.Itoa(len(config)))
	w.WriteHeader(http.StatusOK)
	w.Write(config)
}

// GetServerStatusHandler handles GET /api/status
func (s *Server) handleGetServerStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		status, err := getServerStatus()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get server status: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, status)
	}
}

// ListJobsHandler handles GET /api/jobs
func (s *Server) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	var status *service.JobStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := service.JobStatus(statusStr)
		status = &s
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	jobs := s.certWorker.ListJobs(status, limit)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobs,
	})
}

// Helper methods for Server

func (s *Server) extractUserID(r *http.Request) (uuid.UUID, error) {
	// Path formats: /api/users/:id or /api/users/:id/config
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/users/")
	path = strings.TrimSuffix(path, "/config")

	return uuid.Parse(path)
}

func (s *Server) getActorInfo(ctx context.Context) (string, string) {
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		return claims.UserID.String(), "user"
	}
	return "", "anonymous"
}

// Helper functions

func isValidUsername(name string) bool {
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", name)
	return matched && len(name) >= 1 && len(name) <= 32
}

func getServerStatus() (map[string]interface{}, error) {
	status := map[string]interface{}{
		"online":           false,
		"uptime":           "",
		"connected_users":  0,
		"total_users":      0,
		"server_ip":        "",
		"port":             "1194",
		"protocol":         "udp",
		"version":          "",
		"last_restart":     nil,
	}

	// Check if OpenVPN service is running
	cmd := exec.Command("systemctl", "is-active", "openvpn-server@server")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "active" {
		status["online"] = true
	}

	// Get server IP
	cmd = exec.Command("hostname", "-I")
	output, err = cmd.Output()
	if err == nil {
		ips := strings.Fields(string(output))
		if len(ips) > 0 {
			status["server_ip"] = ips[0]
		}
	}

	// Get connected users count from status file
	connected, err := getConnectedUsersFromStatusFile()
	if err == nil {
		status["connected_users"] = len(connected)
	}

	// Get OpenVPN version
	cmd = exec.Command("openvpn", "--version")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			parts := strings.Fields(lines[0])
			if len(parts) >= 2 {
				status["version"] = parts[1]
			}
		}
	}

	return status, nil
}

func getConnectedUsersFromStatusFile() ([]map[string]interface{}, error) {
	users := []map[string]interface{}{}

	data, err := os.ReadFile("/var/log/openvpn/status.log")
	if err != nil {
		return users, nil // File might not exist if server is not running
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Parse CLIENT_LIST entries
		if strings.HasPrefix(line, "CLIENT_LIST") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 5 {
				user := map[string]interface{}{
					"name":     parts[1],
					"real_ip":  parts[2],
					"virtual_ip": parts[3],
					"status":   "active",
				}
				// Parse connected since time
				if len(parts) >= 8 {
					t, _ := time.Parse("2006-01-02 15:04:05", parts[7])
					if !t.IsZero() {
						user["connected_since"] = t
					}
				}
				users = append(users, user)
			}
		}
	}

	return users, nil
}

// Server methods that delegate to Handler

// ListUsersHandler handles GET /api/users
func (s *Server) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	handler := NewHandler(s.userService, s.jwtManager, s.certWorker, DefaultHandlerConfig())
	handler.ListUsersHandler(w, r)
}

// CreateUserHandler handles POST /api/users
func (s *Server) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	handler := NewHandler(s.userService, s.jwtManager, s.certWorker, DefaultHandlerConfig())
	handler.CreateUserHandler(w, r)
}

// GetUserHandler handles GET /api/users/:id
func (s *Server) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	handler := NewHandler(s.userService, s.jwtManager, s.certWorker, DefaultHandlerConfig())
	handler.GetUserHandler(w, r)
}

// DeleteUserHandler handles DELETE /api/users/:id
func (s *Server) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	handler := NewHandler(s.userService, s.jwtManager, s.certWorker, DefaultHandlerConfig())
	handler.DeleteUserHandler(w, r)
}

// GetUserConfigHandler handles GET /api/users/:id/config
func (s *Server) GetUserConfigHandler(w http.ResponseWriter, r *http.Request) {
	handler := NewHandler(s.userService, s.jwtManager, s.certWorker, DefaultHandlerConfig())
	handler.GetUserConfigHandler(w, r)
}

// Helper methods for Handler

func (h *Handler) extractUserID(r *http.Request) (uuid.UUID, error) {
	// Path formats: /api/users/:id or /api/users/:id/config
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/users/")
	path = strings.TrimSuffix(path, "/config")

	return uuid.Parse(path)
}

func (h *Handler) getActorInfo(ctx context.Context) (string, string) {
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		return claims.UserID.String(), "user"
	}
	return "", "anonymous"
}
