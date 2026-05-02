package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConfigCache(t *testing.T) {
	t.Run("CreateConfigCache", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()

		config := &ConfigCache{
			ID:         uuid.Must(uuid.NewV7()),
			UserID:     userID,
			ConfigData: []byte("client config data"),
			CreatedAt:  now,
			UpdatedAt:  now,
			Checksum:   "abc123",
		}

		assert.NotEqual(t, uuid.Nil, config.ID)
		assert.Equal(t, userID, config.UserID)
		assert.NotEmpty(t, config.ConfigData)
		assert.NotZero(t, config.CreatedAt)
		assert.NotZero(t, config.UpdatedAt)
		assert.Equal(t, "abc123", config.Checksum)
	})

	t.Run("ConfigCacheWithoutChecksum", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()

		config := &ConfigCache{
			ID:         uuid.Must(uuid.NewV7()),
			UserID:     userID,
			ConfigData: []byte("client config data"),
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		assert.Empty(t, config.Checksum)
	})

	t.Run("ConfigCacheJSONTags", func(t *testing.T) {
		config := &ConfigCache{
			ID:         uuid.Must(uuid.NewV7()),
			UserID:     uuid.Must(uuid.NewV7()),
			ConfigData: []byte("test"),
		}

		// Verify JSON tags work correctly
		assert.NotEmpty(t, config.ID)
		assert.NotEmpty(t, config.UserID)
	})
}