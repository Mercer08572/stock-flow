package auth

import (
	"context"
	"time"
)

const (
	DefaultListLimit int32 = 20
	MaxListLimit     int32 = 100
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

type SecretStatus string

const (
	SecretStatusActive  SecretStatus = "active"
	SecretStatusBlocked SecretStatus = "blocked"
)

type CallerType string

const (
	CallerTypeAdmin  CallerType = "admin"
	CallerTypeAPIApp CallerType = "api_app"
)

type Metadata map[string]any

type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AdminIdentity struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type APIApp struct {
	ID          int64     `json:"id"`
	AppID       string    `json:"app_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Status      Status    `json:"status"`
	Metadata    Metadata  `json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type APISecret struct {
	ID            int64        `json:"id"`
	APIAppID      int64        `json:"api_app_id"`
	SecretID      string       `json:"secret_id"`
	SecretHash    string       `json:"-"`
	Name          string       `json:"name"`
	Status        SecretStatus `json:"status"`
	BoundMetadata Metadata     `json:"bound_metadata"`
	ExpiresAt     *time.Time   `json:"expires_at"`
	LastUsedAt    *time.Time   `json:"last_used_at"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Caller struct {
	Type   CallerType    `json:"type"`
	Admin  *AdminCaller  `json:"admin,omitempty"`
	APIApp *APIAppCaller `json:"api_app,omitempty"`
}

type AdminCaller struct {
	AdminUserID int64  `json:"admin_user_id"`
	Username    string `json:"username"`
}

type APIAppCaller struct {
	APIAppID       int64    `json:"api_app_id"`
	AppID          string   `json:"app_id"`
	SecretRecordID int64    `json:"secret_record_id"`
	SecretID       string   `json:"secret_id"`
	AppMetadata    Metadata `json:"app_metadata"`
	BoundMetadata  Metadata `json:"bound_metadata"`
}

type LoginInput struct {
	Username string
	Password string
}

type LoginResult struct {
	Admin     AdminIdentity `json:"admin"`
	Token     string        `json:"-"`
	ExpiresAt time.Time     `json:"expires_at"`
}

type CreateAPIAppInput struct {
	Name        string
	Description *string
	Status      Status
	Metadata    Metadata
}

type UpdateAPIAppInput struct {
	ID          int64
	Name        string
	Description *string
	Status      Status
	Metadata    Metadata
}

type ListFilter struct {
	Status *Status
	Limit  int32
	Offset int32
}

type APIAppListResult struct {
	Items  []APIApp `json:"items"`
	Limit  int32    `json:"limit"`
	Offset int32    `json:"offset"`
}

type IssueSecretInput struct {
	APIAppID      int64
	Name          string
	BoundMetadata Metadata
	ExpiresAt     *time.Time
}

type IssuedSecret struct {
	APISecret
	Secret string `json:"secret"`
}

type Session struct {
	Token       string
	AdminUserID int64
	Username    string
	ExpiresAt   time.Time
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}

func (s SecretStatus) IsValid() bool {
	return s == SecretStatusActive || s == SecretStatusBlocked
}

type callerContextKey struct{}

func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller)
}

func CallerFromContext(ctx context.Context) (Caller, bool) {
	if ctx == nil {
		return Caller{}, false
	}

	caller, ok := ctx.Value(callerContextKey{}).(Caller)
	return caller, ok
}
