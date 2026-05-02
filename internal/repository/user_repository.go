package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/google/uuid"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByName(ctx context.Context, name string) (*models.User, error)
	List(ctx context.Context, filter UserFilter) ([]models.User, error)
	Count(ctx context.Context, filter UserFilter) (int, error)
	Update(ctx context.Context, user *models.User) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.UserStatus) error
	UpdateConnectionInfo(ctx context.Context, id uuid.UUID, virtualIP, realIP string) error
	Delete(ctx context.Context, id uuid.UUID) error
	ExistsByName(ctx context.Context, name string) (bool, error)
}

// UserFilter provides filtering options for user queries
type UserFilter struct {
	Status     *models.UserStatus
	SearchTerm string // Searches name and email
	Limit      int
	Offset     int
}

// userRepository implements UserRepository
type userRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	query := `
		INSERT INTO users (id, name, email, status, created_at, updated_at, cert_serial, cert_expiry)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Name,
		user.Email,
		user.Status,
		user.CreatedAt,
		user.UpdatedAt,
		user.CertSerial,
		user.CertExpiry,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, name, email, status, created_at, updated_at, last_connected, virtual_ip, real_ip, cert_serial, cert_expiry
		FROM users WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanUser(row)
}

func (r *userRepository) GetByName(ctx context.Context, name string) (*models.User, error) {
	query := `
		SELECT id, name, email, status, created_at, updated_at, last_connected, virtual_ip, real_ip, cert_serial, cert_expiry
		FROM users WHERE name = ?
	`
	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanUser(row)
}

func (r *userRepository) List(ctx context.Context, filter UserFilter) ([]models.User, error) {
	query := `
		SELECT id, name, email, status, created_at, updated_at, last_connected, virtual_ip, real_ip, cert_serial, cert_expiry
		FROM users WHERE 1=1
	`
	args := []interface{}{}

	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*filter.Status))
	}

	if filter.SearchTerm != "" {
		query += " AND (name LIKE ? OR email LIKE ?)"
		likeTerm := "%" + filter.SearchTerm + "%"
		args = append(args, likeTerm, likeTerm)
	}

	query += " ORDER BY created_at DESC"

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
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}

	return users, rows.Err()
}

func (r *userRepository) Count(ctx context.Context, filter UserFilter) (int, error) {
	query := "SELECT COUNT(*) FROM users WHERE 1=1"
	args := []interface{}{}

	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*filter.Status))
	}

	if filter.SearchTerm != "" {
		query += " AND (name LIKE ? OR email LIKE ?)"
		likeTerm := "%" + filter.SearchTerm + "%"
		args = append(args, likeTerm, likeTerm)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE users SET
			name = ?,
			email = ?,
			status = ?,
			updated_at = ?,
			last_connected = ?,
			virtual_ip = ?,
			real_ip = ?,
			cert_serial = ?,
			cert_expiry = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		user.Name,
		user.Email,
		user.Status,
		user.UpdatedAt,
		user.LastConnected,
		user.VirtualIP,
		user.RealIP,
		user.CertSerial,
		user.CertExpiry,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.UserStatus) error {
	query := `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateConnectionInfo(ctx context.Context, id uuid.UUID, virtualIP, realIP string) error {
	query := `UPDATE users SET virtual_ip = ?, real_ip = ?, last_connected = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, virtualIP, realIP, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update connection info: %w", err)
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (r *userRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := `SELECT 1 FROM users WHERE name = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return true, nil
}

// Helper methods for scanning users

func (r *userRepository) scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var lastConnected sql.NullTime
	var certExpiry sql.NullTime
	var virtualIP sql.NullString
	var realIP sql.NullString
	var certSerial sql.NullString

	err := row.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
		&lastConnected,
		&virtualIP,
		&realIP,
		&certSerial,
		&certExpiry,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if lastConnected.Valid {
		u.LastConnected = &lastConnected.Time
	}
	if certExpiry.Valid {
		u.CertExpiry = &certExpiry.Time
	}
	if virtualIP.Valid {
		u.VirtualIP = virtualIP.String
	}
	if realIP.Valid {
		u.RealIP = realIP.String
	}
	if certSerial.Valid {
		u.CertSerial = certSerial.String
	}

	return &u, nil
}

func (r *userRepository) scanUserFromRows(rows *sql.Rows) (*models.User, error) {
	var u models.User
	var lastConnected sql.NullTime
	var certExpiry sql.NullTime
	var virtualIP sql.NullString
	var realIP sql.NullString
	var certSerial sql.NullString

	err := rows.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
		&lastConnected,
		&virtualIP,
		&realIP,
		&certSerial,
		&certExpiry,
	)
	if err != nil {
		return nil, err
	}

	if lastConnected.Valid {
		u.LastConnected = &lastConnected.Time
	}
	if certExpiry.Valid {
		u.CertExpiry = &certExpiry.Time
	}
	if virtualIP.Valid {
		u.VirtualIP = virtualIP.String
	}
	if realIP.Valid {
		u.RealIP = realIP.String
	}
	if certSerial.Valid {
		u.CertSerial = certSerial.String
	}

	return &u, nil
}
