package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/google/uuid"
)

// AuditRepository defines the interface for audit log operations
type AuditRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLog, error)
	Count(ctx context.Context, filter models.AuditLogFilter) (int, error)
}

// auditRepository implements AuditRepository
type auditRepository struct {
	db *sql.DB
}

// NewAuditRepository creates a new AuditRepository
func NewAuditRepository(db *sql.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.Must(uuid.NewV7())
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	query := `
		INSERT INTO audit_logs (
			id, timestamp, action, user_id, username, actor_id, actor_type,
			ip_address, user_agent, success, details, duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.ID,
		log.Timestamp,
		log.Action,
		log.UserID,
		log.Username,
		log.ActorID,
		log.ActorType,
		log.IPAddress,
		log.UserAgent,
		log.Success,
		log.Details,
		log.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

func (r *auditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT id, timestamp, action, user_id, username, actor_id, actor_type,
			ip_address, user_agent, success, details, duration_ms
		FROM audit_logs WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanAuditLog(row)
}

func (r *auditRepository) List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLog, error) {
	query := `
		SELECT id, timestamp, action, user_id, username, actor_id, actor_type,
			ip_address, user_agent, success, details, duration_ms
		FROM audit_logs WHERE 1=1
	`
	args := []interface{}{}

	if filter.Action != nil {
		query += " AND action = ?"
		args = append(args, *filter.Action)
	}

	if filter.UserID != nil {
		query += " AND user_id = ?"
		args = append(args, *filter.UserID)
	}

	if filter.ActorID != "" {
		query += " AND actor_id = ?"
		args = append(args, filter.ActorID)
	}

	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.EndTime)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	logs := []models.AuditLog{}
	for rows.Next() {
		log, err := r.scanAuditLogFromRows(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}

	return logs, rows.Err()
}

func (r *auditRepository) Count(ctx context.Context, filter models.AuditLogFilter) (int, error) {
	query := "SELECT COUNT(*) FROM audit_logs WHERE 1=1"
	args := []interface{}{}

	if filter.Action != nil {
		query += " AND action = ?"
		args = append(args, *filter.Action)
	}

	if filter.UserID != nil {
		query += " AND user_id = ?"
		args = append(args, *filter.UserID)
	}

	if filter.ActorID != "" {
		query += " AND actor_id = ?"
		args = append(args, filter.ActorID)
	}

	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.EndTime)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs: %w", err)
	}
	return count, nil
}

// Helper methods

func (r *auditRepository) scanAuditLog(row *sql.Row) (*models.AuditLog, error) {
	var log models.AuditLog
	var details string
	var durationMs sql.NullInt64

	err := row.Scan(
		&log.ID,
		&log.Timestamp,
		&log.Action,
		&log.UserID,
		&log.Username,
		&log.ActorID,
		&log.ActorType,
		&log.IPAddress,
		&log.UserAgent,
		&log.Success,
		&details,
		&durationMs,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	log.Details = details
	if durationMs.Valid {
		log.DurationMs = int(durationMs.Int64)
	}

	return &log, nil
}

func (r *auditRepository) scanAuditLogFromRows(rows *sql.Rows) (*models.AuditLog, error) {
	var log models.AuditLog
	var details string
	var durationMs sql.NullInt64

	err := rows.Scan(
		&log.ID,
		&log.Timestamp,
		&log.Action,
		&log.UserID,
		&log.Username,
		&log.ActorID,
		&log.ActorType,
		&log.IPAddress,
		&log.UserAgent,
		&log.Success,
		&details,
		&durationMs,
	)
	if err != nil {
		return nil, err
	}

	log.Details = details
	if durationMs.Valid {
		log.DurationMs = int(durationMs.Int64)
	}

	return &log, nil
}

// AuditLogContextKey is the context key for audit log correlation
type AuditLogContextKey string

const AuditLogRequestIDKey AuditLogContextKey = "audit_log_request_id"
