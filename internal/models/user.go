package models

import (
	"time"

	"github.com/google/uuid"
)

// UserStatus represents the status of a VPN user
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusRevoked  UserStatus = "revoked"
)

// User represents a VPN user in the system
type User struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	Email         string     `json:"email" db:"email"`
	Status        UserStatus `json:"status" db:"status"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	LastConnected *time.Time `json:"last_connected,omitempty" db:"last_connected"`
	VirtualIP     string     `json:"virtual_ip,omitempty" db:"virtual_ip"`
	RealIP        string     `json:"real_ip,omitempty" db:"real_ip"`
	CertSerial    string     `json:"cert_serial,omitempty" db:"cert_serial"`
	CertExpiry    *time.Time `json:"cert_expiry,omitempty" db:"cert_expiry"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required,alphanum,min=1,max=32"`
	Email string `json:"email,omitempty" validate:"omitempty,email"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email  string     `json:"email,omitempty" validate:"omitempty,email"`
	Status UserStatus `json:"status,omitempty" validate:"omitempty,oneof=active inactive revoked"`
}

// UserListResponse represents a paginated list of users
type UserListResponse struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}

// ConnectedUserInfo represents a currently connected user
type ConnectedUserInfo struct {
	Name          string     `json:"name"`
	RealIP        string     `json:"real_ip"`
	VirtualIP     string     `json:"virtual_ip"`
	ConnectedSince time.Time `json:"connected_since"`
	BytesReceived int64      `json:"bytes_received"`
	BytesSent     int64      `json:"bytes_sent"`
}
