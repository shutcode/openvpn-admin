package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/google/uuid"
)

// JobType represents the type of certificate job
type JobType string

const (
	JobTypeGenerateCert JobType = "generate_cert"
	JobTypeRevokeCert   JobType = "revoke_cert"
	JobTypeRenewCert    JobType = "renew_cert"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// Job represents a certificate operation job
type Job struct {
	ID          uuid.UUID       `json:"id"`
	Type        JobType         `json:"type"`
	Status      JobStatus       `json:"status"`
	UserID      uuid.UUID       `json:"user_id"`
	Username    string          `json:"username"`
	Error       string          `json:"error,omitempty"`
	Result      *JobResult      `json:"result,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// JobResult contains the output of a job
type JobResult struct {
	CertificatePath string `json:"certificate_path,omitempty"`
	KeyPath           string `json:"key_path,omitempty"`
	ConfigPath        string `json:"config_path,omitempty"`
	SerialNumber      string `json:"serial_number,omitempty"`
	Fingerprint       string `json:"fingerprint,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// CertificateWorker handles certificate generation via easy-rsa
type CertificateWorker struct {
	easyRsaPath      string
	openVPNPath      string
	clientsDir       string
	jobs             chan *Job
	results          chan *Job
	jobsMu           sync.RWMutex
	jobsByID         map[uuid.UUID]*Job
	userRepo         repository.UserRepository
	configCache      *ConfigCacheManager
	workerCount      int
	shutdownCh       chan struct{}
	wg               sync.WaitGroup
}

// CertificateWorkerConfig holds configuration for the worker
type CertificateWorkerConfig struct {
	EasyRsaPath string
	OpenVPNPath string
	ClientsDir  string
	WorkerCount int
	QueueSize   int
}

// NewCertificateWorker creates a new certificate worker
func NewCertificateWorker(
	config CertificateWorkerConfig,
	userRepo repository.UserRepository,
	configCache *ConfigCacheManager,
) *CertificateWorker {
	if config.EasyRsaPath == "" {
		config.EasyRsaPath = "/etc/openvpn/easy-rsa"
	}
	if config.OpenVPNPath == "" {
		config.OpenVPNPath = "/etc/openvpn"
	}
	if config.ClientsDir == "" {
		config.ClientsDir = "/etc/openvpn/clients"
	}
	if config.WorkerCount == 0 {
		config.WorkerCount = 2
	}
	if config.QueueSize == 0 {
		config.QueueSize = 100
	}

	return &CertificateWorker{
		easyRsaPath: config.EasyRsaPath,
		openVPNPath: config.OpenVPNPath,
		clientsDir:  config.ClientsDir,
		jobs:        make(chan *Job, config.QueueSize),
		results:     make(chan *Job, config.QueueSize),
		jobsByID:    make(map[uuid.UUID]*Job),
		userRepo:    userRepo,
		configCache: configCache,
		workerCount: config.WorkerCount,
		shutdownCh:  make(chan struct{}),
	}
}

// Start begins processing jobs
func (w *CertificateWorker) Start() {
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}

	// Result collector
	w.wg.Add(1)
	go w.resultCollector()
}

// Stop gracefully shuts down the worker
func (w *CertificateWorker) Stop() {
	close(w.shutdownCh)
	w.wg.Wait()
}

// SubmitJob adds a new job to the queue
func (w *CertificateWorker) SubmitJob(ctx context.Context, jobType JobType, userID uuid.UUID, username string) (*Job, error) {
	job := &Job{
		ID:        uuid.Must(uuid.NewV7()),
		Type:      jobType,
		Status:    JobStatusPending,
		UserID:    userID,
		Username:  username,
		CreatedAt: time.Now().UTC(),
	}

	w.jobsMu.Lock()
	w.jobsByID[job.ID] = job
	w.jobsMu.Unlock()

	select {
	case w.jobs <- job:
		return job, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetJob retrieves a job by ID
func (w *CertificateWorker) GetJob(jobID uuid.UUID) *Job {
	w.jobsMu.RLock()
	defer w.jobsMu.RUnlock()
	return w.jobsByID[jobID]
}

// worker processes jobs
func (w *CertificateWorker) worker(id int) {
	defer w.wg.Done()

	for {
		select {
		case job := <-w.jobs:
			w.processJob(job)
		case <-w.shutdownCh:
			return
		}
	}
}

// processJob handles a single job
func (w *CertificateWorker) processJob(job *Job) {
	now := time.Now().UTC()
	job.Status = JobStatusProcessing
	job.StartedAt = &now

	var err error
	switch job.Type {
	case JobTypeGenerateCert:
		err = w.generateCertificate(job)
	case JobTypeRevokeCert:
		err = w.revokeCertificate(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	completedAt := time.Now().UTC()
	job.CompletedAt = &completedAt

	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = JobStatusCompleted
	}

	w.results <- job
}

// resultCollector handles completed jobs
func (w *CertificateWorker) resultCollector() {
	defer w.wg.Done()

	for {
		select {
		case job := <-w.results:
			w.jobsMu.Lock()
			w.jobsByID[job.ID] = job
			w.jobsMu.Unlock()

			// Update user status based on job result
			if job.Status == JobStatusCompleted {
				ctx := context.Background()
				switch job.Type {
				case JobTypeGenerateCert:
					w.userRepo.UpdateStatus(ctx, job.UserID, models.UserStatusActive)
				case JobTypeRevokeCert:
					w.userRepo.UpdateStatus(ctx, job.UserID, models.UserStatusRevoked)
				}
			}
		case <-w.shutdownCh:
			return
		}
	}
}

// generateCertificate generates a client certificate using easy-rsa
func (w *CertificateWorker) generateCertificate(job *Job) error {
	ctx := context.Background()

	// Ensure client directory exists
	if err := os.MkdirAll(w.clientsDir, 0755); err != nil {
		return fmt.Errorf("failed to create clients directory: %w", err)
	}

	// Build client certificate using easy-rsa
	cmd := exec.CommandContext(ctx,
		"./easyrsa",
		"--batch",
		"--pki-dir="+filepath.Join(w.easyRsaPath, "pki"),
		"--vars="+filepath.Join(w.easyRsaPath, "vars"),
		"build-client-full",
		job.Username,
		"nopass",
	)
	cmd.Dir = w.easyRsaPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the user already exists, that's okay
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("easyrsa failed: %w\nOutput: %s", err, string(output))
		}
	}

	// Generate client config (.ovpn file)
	configPath, err := w.generateClientConfig(job)
	if err != nil {
		return fmt.Errorf("failed to generate client config: %w", err)
	}

	// Extract certificate info
	certPath := filepath.Join(w.easyRsaPath, "pki", "issued", job.Username+".crt")
	serial, fingerprint, expires, err := w.extractCertInfo(certPath)
	if err != nil {
		// Non-fatal: log but continue
		log.Printf("Warning: could not extract cert info: %v", err)
	}

	// Update job result
	job.Result = &JobResult{
		CertificatePath: certPath,
		KeyPath:         filepath.Join(w.easyRsaPath, "pki", "private", job.Username+".key"),
		ConfigPath:      configPath,
		SerialNumber:    serial,
		Fingerprint:     fingerprint,
		ExpiresAt:       expires,
	}

	// Update user with certificate info
	if serial != "" {
		user, err := w.userRepo.GetByID(ctx, job.UserID)
		if err == nil && user != nil {
			user.CertSerial = serial
			user.CertExpiry = expires
			w.userRepo.Update(ctx, user)
		}
	}

	return nil
}

// revokeCertificate revokes a client certificate
func (w *CertificateWorker) revokeCertificate(job *Job) error {
	ctx := context.Background()

	// Revoke certificate using easy-rsa
	cmd := exec.CommandContext(ctx,
		"./easyrsa",
		"--batch",
		"--pki-dir="+filepath.Join(w.easyRsaPath, "pki"),
		"--vars="+filepath.Join(w.easyRsaPath, "vars"),
		"revoke",
		job.Username,
	)
	cmd.Dir = w.easyRsaPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the user doesn't exist, that's an error
		if strings.Contains(string(output), "does not exist") {
			return fmt.Errorf("user %s does not exist", job.Username)
		}
		return fmt.Errorf("easyrsa revoke failed: %w\nOutput: %s", err, string(output))
	}

	// Generate updated CRL
	cmd = exec.CommandContext(ctx,
		"./easyrsa",
		"--batch",
		"--pki-dir="+filepath.Join(w.easyRsaPath, "pki"),
		"gen-crl",
	)
	cmd.Dir = w.easyRsaPath

	if output, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("easyrsa gen-crl failed: %w\nOutput: %s", err, string(output))
	}

	// Remove client config file
	configPath := filepath.Join(w.clientsDir, job.Username+".ovpn")
	os.Remove(configPath)

	return nil
}

// generateClientConfig generates the .ovpn client config file
func (w *CertificateWorker) generateClientConfig(job *Job) (string, error) {
	// Read template
	templatePath := filepath.Join(w.openVPNPath, "client-template.txt")
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read client template: %w", err)
	}

	// Build config
	var config bytes.Buffer
	config.Write(template)
	config.WriteString("\n\n")

	pkiPath := filepath.Join(w.easyRsaPath, "pki")
	userCertPath := filepath.Join(pkiPath, "issued", job.Username+".crt")
	userKeyPath := filepath.Join(pkiPath, "private", job.Username+".key")

	// Add CA cert
	caPath := filepath.Join(pkiPath, "ca.crt")
	config.WriteString("\n<ca\u003e\n")
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return "", fmt.Errorf("failed to read CA cert: %w", err)
	}
	config.Write(caData)
	config.WriteString("\n</ca\u003e\n")

	// Add client cert
	config.WriteString("\n<cert\u003e\n")
	certData, err := os.ReadFile(userCertPath)
	if err != nil {
		return "", fmt.Errorf("failed to read user cert: %w", err)
	}
	// Extract just the certificate part
	config.Write(extractCertBlock(certData))
	config.WriteString("\n</cert\u003e\n")

	// Add client key
	config.WriteString("\n<key\u003e\n")
	keyData, err := os.ReadFile(userKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read user key: %w", err)
	}
	config.Write(keyData)
	config.WriteString("\n</key\u003e\n")

	// Add TLS key if present
	tlsCryptPath := filepath.Join(w.openVPNPath, "tls-crypt.key")
	tlsAuthPath := filepath.Join(w.openVPNPath, "tls-auth.key")

	if _, err := os.Stat(tlsCryptPath); err == nil {
		config.WriteString("\n\u003ctls-crypt\u003e\n")
		tlsData, _ := os.ReadFile(tlsCryptPath)
		config.Write(tlsData)
		config.WriteString("\n\u003c/tls-crypt\u003e\n")
	} else if _, err := os.Stat(tlsAuthPath); err == nil {
		config.WriteString("\nkey-direction 1\n\n\u003ctls-auth\u003e\n")
		tlsData, _ := os.ReadFile(tlsAuthPath)
		config.Write(tlsData)
		config.WriteString("\n\u003c/tls-auth\u003e\n")
	}

	// Write config file
	configPath := filepath.Join(w.clientsDir, job.Username+".ovpn")
	if err := os.WriteFile(configPath, config.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

// extractCertBlock extracts the PEM certificate block from raw certificate data
func extractCertBlock(data []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var output bytes.Buffer
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "BEGIN CERTIFICATE") {
			inBlock = true
		}
		if inBlock {
			output.WriteString(line)
			output.WriteByte('\n')
		}
		if strings.Contains(line, "END CERTIFICATE") {
			break
		}
	}

	return output.Bytes()
}

// extractCertInfo extracts serial, fingerprint, and expiry from a certificate
func (w *CertificateWorker) extractCertInfo(certPath string) (serial, fingerprint string, expires *time.Time, err error) {
	// Use openssl to get certificate info
	cmd := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-serial", "-fingerprint", "-enddate")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", nil, fmt.Errorf("openssl failed: %w\nOutput: %s", err, string(output))
	}

	// Parse output
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "serial=") {
			serial = strings.TrimPrefix(line, "serial=")
		} else if strings.HasPrefix(line, "SHA1 Fingerprint=") {
			fingerprint = strings.TrimPrefix(line, "SHA1 Fingerprint=")
		} else if strings.HasPrefix(line, "notAfter=") {
			dateStr := strings.TrimPrefix(line, "notAfter=")
			// Parse date format: "Dec  7 12:00:00 2024 GMT"
			t, err := time.Parse("Jan  2 15:04:05 2006 MST", dateStr)
			if err == nil {
				expires = &t
			}
		}
	}

	return serial, fingerprint, expires, nil
}


// ListJobs lists jobs with optional filtering
func (w *CertificateWorker) ListJobs(status *JobStatus, limit int) []*Job {
	w.jobsMu.RLock()
	defer w.jobsMu.RUnlock()

	var jobs []*Job
	count := 0

	// Iterate in reverse order (newest first)
	for i := len(w.jobsByID) - 1; i >= 0 && (limit == 0 || count < limit); i-- {
		// This is a simplified version - in production you'd maintain an ordered index
		for _, job := range w.jobsByID {
			if status != nil && job.Status != *status {
				continue
			}
			jobs = append(jobs, job)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	return jobs
}

// ConfigCacheManager manages cached client configurations
type ConfigCacheManager struct {
	clientsDir string
	configRepo ConfigCacheRepo
}

// ConfigCacheRepo interface for config cache operations
type ConfigCacheRepo interface {
	Save(ctx context.Context, userID uuid.UUID, configData []byte) error
	Get(ctx context.Context, userID uuid.UUID) ([]byte, error)
	Delete(ctx context.Context, userID uuid.UUID) error
}

// NewConfigCacheManager creates a new config cache manager
func NewConfigCacheManager(clientsDir string, configRepo ConfigCacheRepo) *ConfigCacheManager {
	return &ConfigCacheManager{
		clientsDir: clientsDir,
		configRepo: configRepo,
	}
}

// CacheConfig caches a generated config
func (m *ConfigCacheManager) CacheConfig(ctx context.Context, userID uuid.UUID, configData []byte) error {
	return m.configRepo.Save(ctx, userID, configData)
}

// GetConfig retrieves a cached config
func (m *ConfigCacheManager) GetConfig(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	return m.configRepo.Get(ctx, userID)
}

// InvalidateCache removes a cached config
func (m *ConfigCacheManager) InvalidateCache(ctx context.Context, userID uuid.UUID) error {
	return m.configRepo.Delete(ctx, userID)
}

// GetConfigFilePath returns the path to a client config file
func (m *ConfigCacheManager) GetConfigFilePath(username string) string {
	return filepath.Join(m.clientsDir, username+".ovpn")
}

// ReadConfigFile reads a client config from disk
func (m *ConfigCacheManager) ReadConfigFile(username string) ([]byte, error) {
	path := m.GetConfigFilePath(username)
	return os.ReadFile(path)
}
