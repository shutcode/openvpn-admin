package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/google/uuid"
)

// ConfigRepository defines the interface for config cache operations
type ConfigRepository interface {
	Save(ctx context.Context, userID uuid.UUID, configData []byte) error
	Get(ctx context.Context, userID uuid.UUID) ([]byte, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.ConfigCache, error)
	Delete(ctx context.Context, userID uuid.UUID) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

// configRepository implements ConfigRepository
type configRepository struct {
	db *sql.DB
}

// NewConfigRepository creates a new ConfigRepository
func NewConfigRepository(db *sql.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) Save(ctx context.Context, userID uuid.UUID, configData []byte) error {
	// Check if config already exists for this user
	var existingID uuid.UUID
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM config_cache WHERE user_id = ?", userID).Scan(&existingID)

	if err == nil {
		// Update existing
		query := `UPDATE config_cache SET config_data = ?, updated_at = ? WHERE id = ?`
		_, err = r.db.ExecContext(ctx, query, configData, time.Now().UTC(), existingID)
		if err != nil {
			return fmt.Errorf("failed to update config cache: %w", err)
		}
	} else if err == sql.ErrNoRows {
		// Insert new
		id := uuid.Must(uuid.NewV7())
		query := `
			INSERT INTO config_cache (id, user_id, config_data, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`
		now := time.Now().UTC()
		_, err = r.db.ExecContext(ctx, query, id, userID, configData, now, now)
		if err != nil {
			return fmt.Errorf("failed to insert config cache: %w", err)
		}
	} else {
		return fmt.Errorf("failed to check existing config: %w", err)
	}

	return nil
}

func (r *configRepository) Get(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	var configData []byte
	query := `SELECT config_data FROM config_cache WHERE user_id = ?`
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&configData)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get config cache: %w", err)
	}
	return configData, nil
}

func (r *configRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.ConfigCache, error) {
	query := `
		SELECT id, user_id, config_data, created_at, updated_at, checksum
		FROM config_cache WHERE user_id = ?
	`
	row := r.db.QueryRowContext(ctx, query, userID)
	return r.scanConfigCache(row)
}

func (r *configRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM config_cache WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete config cache: %w", err)
	}
	return nil
}

func (r *configRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM config_cache WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete config cache: %w", err)
	}
	return nil
}

func (r *configRepository) scanConfigCache(row *sql.Row) (*models.ConfigCache, error) {
	var c models.ConfigCache
	var checksum sql.NullString

	err := row.Scan(
		&c.ID,
		&c.UserID,
		&c.ConfigData,
		&c.CreatedAt,
		&c.UpdatedAt,
		&checksum,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if checksum.Valid {
		c.Checksum = checksum.String
	}

	return &c, nil
}
