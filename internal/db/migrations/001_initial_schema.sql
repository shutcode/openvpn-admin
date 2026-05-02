-- Initial schema for OpenVPN management service

-- Users table: stores VPN user metadata and status
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY, -- UUID v4
    name TEXT NOT NULL UNIQUE, -- Username (matches easy-rsa CN)
    email TEXT,
    status TEXT NOT NULL DEFAULT 'inactive' CHECK (status IN ('active', 'inactive', 'revoked')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_connected DATETIME,
    virtual_ip TEXT,
    real_ip TEXT,
    cert_serial TEXT,
    cert_expiry DATETIME,
    cert_fingerprint TEXT,
    config_cached INTEGER DEFAULT 0 -- boolean: 0 = false, 1 = true
);

-- Index for common queries
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Config cache table: stores generated .ovpn config files
CREATE TABLE IF NOT EXISTS config_cache (
    id TEXT PRIMARY KEY, -- UUID v4
    user_id TEXT NOT NULL,
    config_data BLOB NOT NULL, -- The .ovpn file content
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checksum TEXT, -- SHA256 of config_data for integrity
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_config_user ON config_cache(user_id);

-- Audit logs table: tracks all operations for compliance
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY, -- UUID v4
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL, -- e.g., 'user.create', 'cert.revoke'
    user_id TEXT, -- affected user (nullable)
    username TEXT, -- affected username (nullable)
    actor_id TEXT NOT NULL, -- who performed the action (api_key_id or user)
    actor_type TEXT NOT NULL CHECK (actor_type IN ('api_key', 'user', 'system')),
    ip_address TEXT,
    user_agent TEXT,
    success INTEGER NOT NULL DEFAULT 1, -- boolean: 0 = false, 1 = true
    details TEXT, -- JSON string for additional context
    duration_ms INTEGER -- operation duration in milliseconds
);

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id);

-- API keys table: for service-to-service authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY, -- UUID v4
    name TEXT NOT NULL, -- descriptive name for the key
    key_hash TEXT NOT NULL UNIQUE, -- bcrypt hash of the API key (key itself is shown once)
    permissions TEXT NOT NULL DEFAULT 'read', -- comma-separated: read, write, admin
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    last_used_at DATETIME,
    is_active INTEGER NOT NULL DEFAULT 1, -- boolean: 0 = false, 1 = true
    created_by TEXT -- actor who created this key
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(is_active);

-- Certificate tracking table: detailed cert lifecycle tracking
CREATE TABLE IF NOT EXISTS certificates (
    id TEXT PRIMARY KEY, -- UUID v4
    user_id TEXT NOT NULL,
    serial_number TEXT NOT NULL UNIQUE,
    fingerprint TEXT NOT NULL, -- SHA256 of the cert
    issued_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME,
    revocation_reason INTEGER, -- RFC 5280 reason code
    pem_data TEXT, -- the certificate PEM
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_certs_user ON certificates(user_id);
CREATE INDEX IF NOT EXISTS idx_certs_serial ON certificates(serial_number);
CREATE INDEX IF NOT EXISTS idx_certs_expires ON certificates(expires_at);

-- Triggers for automatic timestamp updates
CREATE TRIGGER IF NOT EXISTS update_users_timestamp
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_config_cache_timestamp
AFTER UPDATE ON config_cache
BEGIN
    UPDATE config_cache SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- View for active users with their latest certificate
CREATE VIEW IF NOT EXISTS v_user_summary AS
SELECT
    u.id,
    u.name,
    u.email,
    u.status,
    u.created_at,
    u.last_connected,
    u.virtual_ip,
    u.real_ip,
    c.serial_number as cert_serial,
    c.expires_at as cert_expires,
    CASE WHEN c.revoked_at IS NOT NULL THEN 1 ELSE 0 END as is_revoked
FROM users u
LEFT JOIN certificates c ON u.id = c.user_id
    AND c.issued_at = (
        SELECT MAX(issued_at) FROM certificates WHERE user_id = u.id
    );
