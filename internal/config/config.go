package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// Database
	DBPath string

	// HTTP Server
	Port string

	// EasyRSA paths
	EasyRSAPath string
	OpenVPNPath string
	ClientsDir  string

	// Security
	JWTSecret string

	// Worker
	WorkerCount int
	QueueSize   int

	// SPA dashboard directory served at /
	DashboardDir string

	// Admin auth (env-var only for now)
	AdminUser     string
	AdminPassword string

	// OpenVPN service unit name and status file location
	StatusFile  string
	ServiceUnit string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Database
		DBPath: getEnv("DB_PATH", "./data/openvpn.db"),

		// HTTP Server
		Port: getEnv("PORT", "8080"),

		// EasyRSA paths
		EasyRSAPath: getEnv("EASYRSA_PATH", "/etc/openvpn/easy-rsa"),
		OpenVPNPath: getEnv("OPENVPN_PATH", "/etc/openvpn"),
		ClientsDir:  getEnv("CLIENTS_DIR", "/etc/openvpn/clients"),

		// Worker
		WorkerCount: getEnvInt("WORKER_COUNT", 2),
		QueueSize:   getEnvInt("QUEUE_SIZE", 100),

		// Dashboard
		DashboardDir: getEnv("DASHBOARD_DIR", "./dashboard"),

		// Admin auth
		AdminUser:     getEnv("ADMIN_USER", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),

		// OpenVPN runtime
		StatusFile:  getEnv("STATUS_FILE", "/var/log/openvpn/status.log"),
		ServiceUnit: getEnv("SERVICE_UNIT", "openvpn-server@server.service"),
	}

	// JWT Secret - required
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Try to read from file
		jwtSecretFile := os.Getenv("JWT_SECRET_FILE")
		if jwtSecretFile != "" {
			data, err := os.ReadFile(jwtSecretFile)
			if err != nil {
				return nil, fmt.Errorf("JWT_SECRET not set and failed to read JWT_SECRET_FILE: %w", err)
			}
			jwtSecret = string(data)
		}
	}

	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET (or JWT_SECRET_FILE) is required; generate one with: openssl rand -hex 32")
	}

	cfg.JWTSecret = jwtSecret

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// best-effort — server will run even if it can't pre-create the dir
	// (the manager handles MkdirAll lazily before writing).
	_ = os.MkdirAll(cfg.ClientsDir, 0o755)

	return cfg, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as an integer with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
