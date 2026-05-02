package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shutcode/openvpn-admin/internal/models"
	"github.com/shutcode/openvpn-admin/internal/repository"
	"github.com/google/uuid"
)

// UserService handles high-level user operations
type UserService struct {
	userRepo    repository.UserRepository
	auditRepo   repository.AuditRepository
	certWorker  *CertificateWorker
	configCache *ConfigCacheManager
}

// NewUserService creates a new UserService
func NewUserService(
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	certWorker *CertificateWorker,
	configCache *ConfigCacheManager,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		auditRepo:   auditRepo,
		certWorker:  certWorker,
		configCache: configCache,
	}
}

// CreateUser creates a new user and initiates certificate generation
func (s *UserService) CreateUser(ctx context.Context, req models.CreateUserRequest, actorID, actorType string) (*models.User, error) {
	start := time.Now()

	// Check if username already exists
	exists, err := s.userRepo.ExistsByName(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("user with name '%s' already exists", req.Name)
	}

	// Create user record
	user := &models.User{
		ID:       uuid.Must(uuid.NewV7()),
		Name:     req.Name,
		Email:    req.Email,
		Status:   models.UserStatusInactive, // Will be activated when cert is ready
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Log audit event
	duration := time.Since(start)
	s.logAudit(ctx, models.AuditActionUserCreate, user.ID, user.Name, actorID, actorType, true, "", duration)

	// Submit certificate generation job
	_, err = s.certWorker.SubmitJob(ctx, JobTypeGenerateCert, user.ID, user.Name)
	if err != nil {
		// Log but don't fail - user is created, cert will be pending
		s.logAudit(ctx, models.AuditActionCertGenerate, user.ID, user.Name, actorID, actorType, false, err.Error(), 0)
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

// GetUserByName retrieves a user by name
func (s *UserService) GetUserByName(ctx context.Context, name string) (*models.User, error) {
	return s.userRepo.GetByName(ctx, name)
}

// ListUsers retrieves a paginated list of users
func (s *UserService) ListUsers(ctx context.Context, filter repository.UserFilter) (*models.UserListResponse, error) {
	users, err := s.userRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	count, err := s.userRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.UserListResponse{
		Users: users,
		Total: count,
	}, nil
}

// DeleteUser revokes certificates and marks user as deleted
func (s *UserService) DeleteUser(ctx context.Context, userID uuid.UUID, actorID, actorType string) error {
	start := time.Now()

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Submit certificate revocation job
	_, err = s.certWorker.SubmitJob(ctx, JobTypeRevokeCert, userID, user.Name)
	if err != nil {
		// Log but continue - we still want to mark user as revoked
		s.logAudit(ctx, models.AuditActionCertRevoke, userID, user.Name, actorID, actorType, false, err.Error(), 0)
	}

	// Update user status
	if err := s.userRepo.UpdateStatus(ctx, userID, models.UserStatusRevoked); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Invalidate config cache
	if err := s.configCache.InvalidateCache(ctx, userID); err != nil {
		// Log but don't fail
		// fmt.Printf("Failed to invalidate config cache: %v\n", err)
	}

	duration := time.Since(start)
	s.logAudit(ctx, models.AuditActionUserDelete, userID, user.Name, actorID, actorType, true, "", duration)

	return nil
}

// GetUserConfig retrieves the OpenVPN config for a user
func (s *UserService) GetUserConfig(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	// First try the cache
	config, err := s.configCache.GetConfig(ctx, userID)
	if err == nil && config != nil {
		return config, nil
	}

	// Get user to find the username
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Read from disk
	config, err = s.configCache.ReadConfigFile(user.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Cache for future requests
	s.configCache.CacheConfig(ctx, userID, config)

	return config, nil
}

// logAudit creates an audit log entry
func (s *UserService) logAudit(ctx context.Context, action models.AuditAction, userID uuid.UUID, username, actorID, actorType string, success bool, details string, duration time.Duration) {
	log := &models.AuditLog{
		Action:    action,
		UserID:    &userID,
		Username:  username,
		ActorID:   actorID,
		ActorType: actorType,
		Success:   success,
		Details:   details,
		DurationMs: int(duration.Milliseconds()),
	}

	if err := s.auditRepo.Create(ctx, log); err != nil {
		// Log error but don't fail the operation
		// fmt.Printf("Failed to create audit log: %v\n", err)
	}
}
