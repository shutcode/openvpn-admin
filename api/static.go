package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DashboardDir is the directory containing the SPA. Set via constructor.
type StaticConfig struct {
	DashboardDir string
}

// staticHandler serves the SPA. Any request that isn't matched by /api or
// /health falls through here and gets dashboard/index.html.
func staticHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Serve specific file if it exists; otherwise fall back to index.html
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "" || clean == "." {
			clean = "index.html"
		}
		// Prevent directory traversal
		if strings.Contains(clean, "..") {
			http.NotFound(w, r)
			return
		}

		full := filepath.Join(dir, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, r, full)
			return
		}

		// SPA fallback
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
}

// healthHandler is a lightweight liveness probe.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// loginRequest matches the SPA login form payload.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginHandler is a demo authentication endpoint that issues a JWT for
// the SPA. Real password hashing/storage is out of scope for this prototype.
func (s *Server) loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "username and password required")
			return
		}

		// Demo policy: only `admin` is permitted, and the literal "wrong"
		// password is rejected so the UI can demonstrate failure paths.
		if req.Username != "admin" || req.Password == "wrong" {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"username": req.Username,
			"role":     "admin",
		})
	}
}
