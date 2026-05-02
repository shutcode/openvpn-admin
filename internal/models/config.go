package models

import (
	"time"

	"github.com/google/uuid"
)

// ConfigCache represents a cached OpenVPN client configuration
type ConfigCache struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	ConfigData []byte    `json:"config_data" db:"config_data"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	Checksum   string    `json:"checksum,omitempty" db:"checksum"`
}
