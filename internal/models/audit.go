package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of audit action
type AuditAction string

const (
	AuditActionUserCreate   AuditAction = "user.create"
	AuditActionUserUpdate   AuditAction = "user.update"
	AuditActionUserDelete   AuditAction = "user.delete"
	AuditActionUserRevoke   AuditAction = "user.revoke"
	AuditActionCertGenerate AuditAction = "cert.generate"
	AuditActionCertRevoke   AuditAction = "cert.revoke"
	AuditActionConfigExport AuditAction = "config.export"
	AuditActionLogin        AuditAction = "auth.login"
	AuditActionLogout       AuditAction = "auth.logout"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	Timestamp time.Time   `json:"timestamp" db:"timestamp"`
	Action    AuditAction `json:"action" db:"action"`
	UserID    *uuid.UUID  `json:"user_id,omitempty" db:"user_id"`
	Username  string      `json:"username,omitempty" db:"username"`
	ActorID   string      `json:"actor_id" db:"actor_id"` // API key or user identity
	ActorType string      `json:"actor_type" db:"actor_type"` // "api_key", "user", "system"
	IPAddress string      `json:"ip_address" db:"ip_address"`
	UserAgent string      `json:"user_agent" db:"user_agent"`
	Success   bool        `json:"success" db:"success"`
	Details   string      `json:"details,omitempty" db:"details"` // JSON string for additional context
	DurationMs int      `json:"duration_ms,omitempty" db:"duration_ms"`
}

// CreateAuditLogRequest represents a request to create an audit log
type CreateAuditLogRequest struct {
	Action    AuditAction `json:"action" validate:"required"`
	UserID    *uuid.UUID  `json:"user_id,omitempty"`
	Username  string      `json:"username,omitempty"`
	ActorID   string      `json:"actor_id" validate:"required"`
	ActorType string      `json:"actor_type" validate:"required,oneof=api_key user system"`
	IPAddress string      `json:"ip_address"`
	UserAgent string      `json:"user_agent"`
	Success   bool        `json:"success"`
	Details   string      `json:"details,omitempty"`
}

// AuditLogListResponse represents a paginated list of audit logs
type AuditLogListResponse struct {
	Logs  []AuditLog `json:"logs"`
	Total int        `json:"total"`
}

// AuditLogFilter represents filters for querying audit logs
type AuditLogFilter struct {
	Action    *AuditAction `json:"action,omitempty"`
	UserID    *uuid.UUID   `json:"user_id,omitempty"`
	ActorID   string       `json:"actor_id,omitempty"`
	StartTime *time.Time   `json:"start_time,omitempty"`
	EndTime   *time.Time   `json:"end_time,omitempty"`
	Limit     int          `json:"limit,omitempty"`
	Offset    int          `json:"offset,omitempty"`
}
