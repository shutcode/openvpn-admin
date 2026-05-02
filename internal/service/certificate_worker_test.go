package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shutcode/openvpn-admin/internal/db"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCertWorkerTest(t *testing.T) (*CertificateWorker, func()) {
	tempDir, err := os.MkdirTemp("", "openvpn-worker-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)

	// Create minimal user repo
	userRepo := repository.NewUserRepository(database.DB)
	configRepo := repository.NewConfigRepository(database.DB)
	configCache := NewConfigCacheManager(tempDir, configRepo)

	config := CertificateWorkerConfig{
		EasyRsaPath: "/etc/openvpn/easy-rsa",
		OpenVPNPath: "/etc/openvpn",
		ClientsDir:  tempDir,
		WorkerCount: 1,
		QueueSize:   10,
	}

	worker := NewCertificateWorker(config, userRepo, configCache)

	cleanup := func() {
		worker.Stop()
		database.Close()
		os.RemoveAll(tempDir)
	}

	return worker, cleanup
}

func TestCertificateWorker_New(t *testing.T) {
	worker, cleanup := setupCertWorkerTest(t)
	defer cleanup()

	t.Run("CreateWorker", func(t *testing.T) {
		assert.NotNil(t, worker)
		assert.NotNil(t, worker.jobs)
		assert.NotNil(t, worker.results)
		assert.NotNil(t, worker.jobsByID)
		assert.Equal(t, 1, worker.workerCount)
	})
}

func TestCertificateWorker_SubmitJob(t *testing.T) {
	worker, cleanup := setupCertWorkerTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SubmitGenerateCertJob", func(t *testing.T) {
		job, err := worker.SubmitJob(ctx, JobTypeGenerateCert, uuid.Must(uuid.NewV7()), "testuser")

		assert.NoError(t, err)
		assert.NotNil(t, job)
		assert.Equal(t, JobTypeGenerateCert, job.Type)
		assert.Equal(t, JobStatusPending, job.Status)
		assert.NotEqual(t, uuid.Nil, job.ID)
	})

	t.Run("SubmitRevokeCertJob", func(t *testing.T) {
		job, err := worker.SubmitJob(ctx, JobTypeRevokeCert, uuid.Must(uuid.NewV7()), "testuser")

		assert.NoError(t, err)
		assert.NotNil(t, job)
		assert.Equal(t, JobTypeRevokeCert, job.Type)
	})
}

func TestCertificateWorker_StartStop(t *testing.T) {
	worker, cleanup := setupCertWorkerTest(t)
	defer cleanup()

	t.Run("StartWorker", func(t *testing.T) {
		worker.Start()

		// Give worker time to start
		time.Sleep(100 * time.Millisecond)

		// Verify workers are running (check internal state)
		assert.NotNil(t, worker.wg)
	})
}

func TestCertificateWorker_GetJob(t *testing.T) {
	worker, cleanup := setupCertWorkerTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetExistingJob", func(t *testing.T) {
		job, err := worker.SubmitJob(ctx, JobTypeGenerateCert, uuid.Must(uuid.NewV7()), "testuser")
		require.NoError(t, err)

		retrieved := worker.GetJob(job.ID)

		assert.NotNil(t, retrieved)
		assert.Equal(t, job.ID, retrieved.ID)
	})

	t.Run("GetNonExistentJob", func(t *testing.T) {
		retrieved := worker.GetJob(uuid.Must(uuid.NewV7()))

		assert.Nil(t, retrieved)
	})
}

func TestCertificateWorker_JobTypes(t *testing.T) {
	t.Run("JobTypeConstants", func(t *testing.T) {
		assert.Equal(t, "generate_cert", string(JobTypeGenerateCert))
		assert.Equal(t, "revoke_cert", string(JobTypeRevokeCert))
		assert.Equal(t, "renew_cert", string(JobTypeRenewCert))
	})

	t.Run("JobStatusConstants", func(t *testing.T) {
		assert.Equal(t, "pending", string(JobStatusPending))
		assert.Equal(t, "processing", string(JobStatusProcessing))
		assert.Equal(t, "completed", string(JobStatusCompleted))
		assert.Equal(t, "failed", string(JobStatusFailed))
	})
}

func TestJobStruct(t *testing.T) {
	t.Run("CreateJob", func(t *testing.T) {
		job := &Job{
			ID:       uuid.Must(uuid.NewV7()),
			Type:     JobTypeGenerateCert,
			Status:   JobStatusPending,
			UserID:   uuid.Must(uuid.NewV7()),
			Username: "testuser",
			CreatedAt: time.Now().UTC(),
		}

		assert.NotEqual(t, uuid.Nil, job.ID)
		assert.Equal(t, JobTypeGenerateCert, job.Type)
		assert.Equal(t, JobStatusPending, job.Status)
		assert.NotZero(t, job.CreatedAt)
	})
}

func TestJobResult(t *testing.T) {
	t.Run("CreateJobResult", func(t *testing.T) {
		result := &JobResult{
			CertificatePath: "/etc/openvpn/easy-rsa/pki/issued/testuser.crt",
			KeyPath:         "/etc/openvpn/easy-rsa/pki/private/testuser.key",
			ConfigPath:      "/etc/openvpn/clients/testuser.ovpn",
			SerialNumber:    "ABC123",
			Fingerprint:     "SHA1:ABC:DEF",
		}

		assert.NotEmpty(t, result.CertificatePath)
		assert.NotEmpty(t, result.ConfigPath)
		assert.Equal(t, "ABC123", result.SerialNumber)
	})
}

func TestCertificateWorker_ListJobs(t *testing.T) {
	worker, cleanup := setupCertWorkerTest(t)
	defer cleanup()

	ctx := context.Background()

	// Submit some jobs
	for i := 0; i < 3; i++ {
		_, err := worker.SubmitJob(ctx, JobTypeGenerateCert, uuid.Must(uuid.NewV7()), "user"+string(rune('a'+i)))
		require.NoError(t, err)
	}

	t.Run("ListAllJobs", func(t *testing.T) {
		jobs := worker.ListJobs(nil, 0)

		assert.GreaterOrEqual(t, len(jobs), 3)
	})

	t.Run("ListJobsWithLimit", func(t *testing.T) {
		jobs := worker.ListJobs(nil, 2)

		assert.LessOrEqual(t, len(jobs), 2)
	})
}

func TestConfigCacheManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openvpn-cache-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configRepo := &mockConfigRepo{}
	configCache := NewConfigCacheManager(tempDir, configRepo)

	t.Run("CacheConfig", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		err := configCache.CacheConfig(ctx, userID, configData)

		assert.NoError(t, err)
	})

	t.Run("GetConfig", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		// First cache it
		err := configCache.CacheConfig(ctx, userID, configData)
		require.NoError(t, err)

		// Then retrieve it
		retrieved, err := configCache.GetConfig(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, configData, retrieved)
	})

	t.Run("InvalidateCache", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.Must(uuid.NewV7())
		configData := []byte("test config data")

		// Cache it
		err := configCache.CacheConfig(ctx, userID, configData)
		require.NoError(t, err)

		// Invalidate it
		err = configCache.InvalidateCache(ctx, userID)

		assert.NoError(t, err)
	})

	t.Run("GetConfigFilePath", func(t *testing.T) {
		path := configCache.GetConfigFilePath("testuser")

		assert.Equal(t, filepath.Join(tempDir, "testuser.ovpn"), path)
	})
}

// mockConfigRepo implements ConfigCacheRepo for testing
type mockConfigRepo struct {
	data map[uuid.UUID][]byte
	mu   sync.Mutex
}

func (m *mockConfigRepo) Save(ctx context.Context, userID uuid.UUID, configData []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[uuid.UUID][]byte)
	}
	m.data[userID] = configData
	return nil
}

func (m *mockConfigRepo) Get(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		return nil, nil
	}
	data, ok := m.data[userID]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func (m *mockConfigRepo) Delete(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data != nil {
		delete(m.data, userID)
	}
	return nil
}

func TestNewConfigCacheManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openvpn-newcache-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configRepo := &mockConfigRepo{}
	manager := NewConfigCacheManager(tempDir, configRepo)

	assert.NotNil(t, manager)
	assert.Equal(t, tempDir, manager.clientsDir)
}