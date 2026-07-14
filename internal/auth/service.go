package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultSessionTTL = 8 * time.Hour

type Authenticator interface {
	AuthenticateAdminSession(ctx context.Context, token string) (Caller, error)
	AuthenticateAPIApp(ctx context.Context, appID string, secret string) (Caller, error)
}

type Service interface {
	Authenticator
	Login(ctx context.Context, input LoginInput) (LoginResult, error)
	Logout(ctx context.Context, token string) error
	Me(ctx context.Context, token string) (AdminIdentity, error)
	ChangePassword(ctx context.Context, input ChangePasswordInput) (LoginResult, error)
	InitializeAdminPassword(ctx context.Context, username string, password string) error
	ListAPIApps(ctx context.Context, filter ListFilter) (APIAppListResult, error)
	GetAPIApp(ctx context.Context, id int64) (*APIApp, error)
	CreateAPIApp(ctx context.Context, input CreateAPIAppInput) (*APIApp, error)
	UpdateAPIApp(ctx context.Context, input UpdateAPIAppInput) (*APIApp, error)
	DeleteAPIApp(ctx context.Context, id int64) error
	IssueAPISecret(ctx context.Context, input IssueSecretInput) (*IssuedSecret, error)
	ListAPISecrets(ctx context.Context, apiAppID int64) ([]APISecret, error)
	BlockAPISecret(ctx context.Context, apiAppID int64, secretID string) (*APISecret, error)
}

type Repository interface {
	GetAdminByUsername(ctx context.Context, username string) (*AdminUser, error)
	GetAdminByID(ctx context.Context, id int64) (*AdminUser, error)
	InitializeAdminPassword(ctx context.Context, username string, passwordHash string) error
	ChangeAdminPassword(ctx context.Context, id int64, passwordHash string, changedAt time.Time) error
	ListAPIApps(ctx context.Context, filter ListFilter) ([]APIApp, error)
	GetAPIAppByID(ctx context.Context, id int64) (*APIApp, error)
	GetAPIAppByIdentifier(ctx context.Context, appID string) (*APIApp, error)
	CreateAPIApp(ctx context.Context, appID string, input CreateAPIAppInput) (*APIApp, error)
	UpdateAPIApp(ctx context.Context, input UpdateAPIAppInput) (*APIApp, error)
	SoftDeleteAPIApp(ctx context.Context, id int64) error
	CreateAPISecret(ctx context.Context, secretID string, secretHash string, input IssueSecretInput) (*APISecret, error)
	ListAPISecrets(ctx context.Context, apiAppID int64) ([]APISecret, error)
	ListAPISecretsForAuthentication(ctx context.Context, apiAppID int64) ([]APISecret, error)
	BlockAPISecret(ctx context.Context, apiAppID int64, secretID string) (*APISecret, error)
	TouchAPISecretLastUsed(ctx context.Context, id int64, usedAt time.Time) error
}

type RandomGenerator interface {
	Generate(prefix string, byteLength int) (string, error)
}

type ServiceOptions struct {
	SessionTTL         time.Duration
	Now                func() time.Time
	Random             RandomGenerator
	LoginRateLimiter   LoginRateLimiter
	LoginFailurePolicy RateLimitPolicy
	LoginIPPolicy      RateLimitPolicy
}

type service struct {
	repo               Repository
	sessions           SessionStore
	passwords          PasswordHasher
	sessionTTL         time.Duration
	now                func() time.Time
	random             RandomGenerator
	loginRateLimiter   LoginRateLimiter
	loginFailurePolicy RateLimitPolicy
	loginIPPolicy      RateLimitPolicy
}

func NewService(repo Repository, sessions SessionStore, passwords PasswordHasher, options ServiceOptions) Service {
	if options.SessionTTL <= 0 {
		options.SessionTTL = defaultSessionTTL
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Random == nil {
		options.Random = cryptoRandomGenerator{}
	}
	if options.LoginRateLimiter == nil {
		options.LoginRateLimiter = NewInMemoryLoginRateLimiter(options.Now)
	}
	if options.LoginFailurePolicy.MaxAttempts <= 0 {
		options.LoginFailurePolicy = RateLimitPolicy{MaxAttempts: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute}
	}
	if options.LoginIPPolicy.MaxAttempts <= 0 {
		options.LoginIPPolicy = RateLimitPolicy{MaxAttempts: 30, Window: 15 * time.Minute, Lockout: 15 * time.Minute}
	}

	return &service{
		repo:               repo,
		sessions:           sessions,
		passwords:          passwords,
		sessionTTL:         options.SessionTTL,
		now:                options.Now,
		random:             options.Random,
		loginRateLimiter:   options.LoginRateLimiter,
		loginFailurePolicy: options.LoginFailurePolicy,
		loginIPPolicy:      options.LoginIPPolicy,
	}
}

func (s *service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	input.Username = strings.TrimSpace(input.Username)
	normalizedUsername := strings.ToLower(input.Username)
	clientIP := strings.TrimSpace(input.ClientIP)
	failureKey := "login:" + clientIP + ":" + normalizedUsername
	ipKey := "login-ip:" + clientIP
	if retryAfter, err := s.loginRateLimiter.Check(ctx, ipKey, s.loginIPPolicy); err != nil {
		return LoginResult{}, err
	} else if retryAfter > 0 {
		return LoginResult{}, &RateLimitError{RetryAfter: retryAfter}
	}
	if retryAfter, err := s.loginRateLimiter.Check(ctx, failureKey, s.loginFailurePolicy); err != nil {
		return LoginResult{}, err
	} else if retryAfter > 0 {
		return LoginResult{}, &RateLimitError{RetryAfter: retryAfter}
	}
	if err := s.loginRateLimiter.Record(ctx, ipKey, s.loginIPPolicy); err != nil {
		return LoginResult{}, err
	}
	if input.Username == "" || input.Password == "" {
		_ = s.loginRateLimiter.Record(ctx, failureKey, s.loginFailurePolicy)
		return LoginResult{}, ErrInvalidCredentials
	}

	admin, err := s.repo.GetAdminByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, ErrAdminNotFound) {
			_ = s.loginRateLimiter.Record(ctx, failureKey, s.loginFailurePolicy)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if admin.Status != StatusActive {
		_ = s.loginRateLimiter.Record(ctx, failureKey, s.loginFailurePolicy)
		return LoginResult{}, ErrInvalidCredentials
	}
	if !admin.PasswordInitialized {
		_ = s.loginRateLimiter.Record(ctx, failureKey, s.loginFailurePolicy)
		return LoginResult{}, ErrInvalidCredentials
	}

	valid, err := s.passwords.Verify(input.Password, admin.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify admin password: %w", err)
	}
	if !valid {
		_ = s.loginRateLimiter.Record(ctx, failureKey, s.loginFailurePolicy)
		return LoginResult{}, ErrInvalidCredentials
	}
	_ = s.loginRateLimiter.Reset(ctx, failureKey)
	return s.issueSession(ctx, admin)
}

func (s *service) issueSession(ctx context.Context, admin *AdminUser) (LoginResult, error) {
	token, err := s.random.Generate("sess_", 32)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate admin session: %w", err)
	}

	now := s.now()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.sessions.Save(ctx, Session{
		Token:              token,
		AdminUserID:        admin.ID,
		Username:           admin.Username,
		MustChangePassword: admin.MustChangePassword,
		ExpiresAt:          expiresAt,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("save admin session: %w", err)
	}

	return LoginResult{
		Admin:     AdminIdentity{ID: admin.ID, Username: admin.Username, MustChangePassword: admin.MustChangePassword},
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *service) InitializeAdminPassword(ctx context.Context, username string, password string) error {
	username = strings.TrimSpace(username)
	if err := validateNewPassword(username, password); err != nil {
		return err
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return fmt.Errorf("hash initial admin password: %w", err)
	}
	return s.repo.InitializeAdminPassword(ctx, username, hash)
}

func (s *service) ChangePassword(ctx context.Context, input ChangePasswordInput) (LoginResult, error) {
	caller, err := s.AuthenticateAdminSession(ctx, input.Token)
	if err != nil {
		return LoginResult{}, err
	}
	admin, err := s.repo.GetAdminByID(ctx, caller.Admin.AdminUserID)
	if err != nil {
		return LoginResult{}, err
	}
	valid, err := s.passwords.Verify(input.CurrentPassword, admin.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify current admin password: %w", err)
	}
	if !valid {
		return LoginResult{}, ErrInvalidCredentials
	}
	if same, err := s.passwords.Verify(input.NewPassword, admin.PasswordHash); err != nil {
		return LoginResult{}, fmt.Errorf("compare new admin password: %w", err)
	} else if same {
		return LoginResult{}, NewValidationError("new password must differ from current password")
	}
	if err := validateNewPassword(admin.Username, input.NewPassword); err != nil {
		return LoginResult{}, err
	}
	hash, err := s.passwords.Hash(input.NewPassword)
	if err != nil {
		return LoginResult{}, fmt.Errorf("hash new admin password: %w", err)
	}
	changedAt := s.now()
	if err := s.repo.ChangeAdminPassword(ctx, admin.ID, hash, changedAt); err != nil {
		return LoginResult{}, err
	}
	if err := s.sessions.DeleteByAdminUserID(ctx, admin.ID); err != nil {
		return LoginResult{}, err
	}
	admin.PasswordHash = hash
	admin.MustChangePassword = false
	admin.PasswordChangedAt = &changedAt
	return s.issueSession(ctx, admin)
}

func validateNewPassword(username string, password string) error {
	length := len([]rune(password))
	if length < 12 || length > 128 {
		return NewValidationError("password must contain 12 to 128 characters")
	}
	if strings.EqualFold(username, password) {
		return NewValidationError("password must not equal username")
	}
	return nil
}

func (s *service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrUnauthorized
	}
	return s.sessions.Delete(ctx, token)
}

func (s *service) Me(ctx context.Context, token string) (AdminIdentity, error) {
	caller, err := s.AuthenticateAdminSession(ctx, token)
	if err != nil {
		return AdminIdentity{}, err
	}
	return AdminIdentity{ID: caller.Admin.AdminUserID, Username: caller.Admin.Username, MustChangePassword: caller.Admin.MustChangePassword}, nil
}

func (s *service) AuthenticateAdminSession(ctx context.Context, token string) (Caller, error) {
	if strings.TrimSpace(token) == "" {
		return Caller{}, ErrUnauthorized
	}

	session, ok, err := s.sessions.Get(ctx, token)
	if err != nil {
		return Caller{}, err
	}
	if !ok {
		return Caller{}, ErrUnauthorized
	}
	if !s.now().Before(session.ExpiresAt) {
		_ = s.sessions.Delete(ctx, token)
		return Caller{}, ErrSessionExpired
	}

	return Caller{
		Type: CallerTypeAdmin,
		Admin: &AdminCaller{
			AdminUserID:        session.AdminUserID,
			Username:           session.Username,
			MustChangePassword: session.MustChangePassword,
		},
	}, nil
}

func (s *service) AuthenticateAPIApp(ctx context.Context, appID string, secret string) (Caller, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" || secret == "" {
		return Caller{}, ErrUnauthorized
	}

	app, err := s.repo.GetAPIAppByIdentifier(ctx, appID)
	if err != nil {
		if errors.Is(err, ErrAPIAppNotFound) {
			return Caller{}, ErrInvalidCredentials
		}
		return Caller{}, err
	}
	if app.Status != StatusActive {
		return Caller{}, ErrInvalidCredentials
	}

	secrets, err := s.repo.ListAPISecretsForAuthentication(ctx, app.ID)
	if err != nil {
		return Caller{}, err
	}

	presentedHash := hashAPISecret(secret)
	now := s.now()
	for _, candidate := range secrets {
		if candidate.Status != SecretStatusActive {
			continue
		}
		if candidate.ExpiresAt != nil && !now.Before(*candidate.ExpiresAt) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(presentedHash), []byte(candidate.SecretHash)) != 1 {
			continue
		}

		if err := s.repo.TouchAPISecretLastUsed(ctx, candidate.ID, now); err != nil {
			return Caller{}, err
		}

		return Caller{
			Type: CallerTypeAPIApp,
			APIApp: &APIAppCaller{
				APIAppID:       app.ID,
				AppID:          app.AppID,
				SecretRecordID: candidate.ID,
				SecretID:       candidate.SecretID,
				AppMetadata:    cloneMetadata(app.Metadata),
				BoundMetadata:  cloneMetadata(candidate.BoundMetadata),
			},
		}, nil
	}

	return Caller{}, ErrInvalidCredentials
}

func (s *service) ListAPIApps(ctx context.Context, filter ListFilter) (APIAppListResult, error) {
	normalized, err := normalizeListFilter(filter)
	if err != nil {
		return APIAppListResult{}, err
	}

	items, err := s.repo.ListAPIApps(ctx, normalized)
	if err != nil {
		return APIAppListResult{}, err
	}
	return APIAppListResult{Items: items, Limit: normalized.Limit, Offset: normalized.Offset}, nil
}

func (s *service) GetAPIApp(ctx context.Context, id int64) (*APIApp, error) {
	if id <= 0 {
		return nil, NewValidationError("api app id must be greater than zero")
	}
	return s.repo.GetAPIAppByID(ctx, id)
}

func (s *service) CreateAPIApp(ctx context.Context, input CreateAPIAppInput) (*APIApp, error) {
	normalized, err := normalizeCreateAPIAppInput(input)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 3; attempt++ {
		appID, err := s.random.Generate("app_", 18)
		if err != nil {
			return nil, fmt.Errorf("generate api app identifier: %w", err)
		}
		app, err := s.repo.CreateAPIApp(ctx, appID, normalized)
		if errors.Is(err, ErrDuplicateAppID) {
			continue
		}
		return app, err
	}
	return nil, ErrDuplicateAppID
}

func (s *service) UpdateAPIApp(ctx context.Context, input UpdateAPIAppInput) (*APIApp, error) {
	normalized, err := normalizeUpdateAPIAppInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateAPIApp(ctx, normalized)
}

func (s *service) DeleteAPIApp(ctx context.Context, id int64) error {
	if id <= 0 {
		return NewValidationError("api app id must be greater than zero")
	}
	return s.repo.SoftDeleteAPIApp(ctx, id)
}

func (s *service) IssueAPISecret(ctx context.Context, input IssueSecretInput) (*IssuedSecret, error) {
	normalized, err := normalizeIssueSecretInput(input, s.now())
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetAPIAppByID(ctx, normalized.APIAppID); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 3; attempt++ {
		secretID, err := s.random.Generate("key_", 12)
		if err != nil {
			return nil, fmt.Errorf("generate api secret identifier: %w", err)
		}
		plainSecret, err := s.random.Generate("sfsec_", 32)
		if err != nil {
			return nil, fmt.Errorf("generate api secret: %w", err)
		}

		secret, err := s.repo.CreateAPISecret(ctx, secretID, hashAPISecret(plainSecret), normalized)
		if errors.Is(err, ErrDuplicateSecretID) {
			continue
		}
		if err != nil {
			return nil, err
		}

		return &IssuedSecret{APISecret: *secret, Secret: plainSecret}, nil
	}
	return nil, ErrDuplicateSecretID
}

func (s *service) ListAPISecrets(ctx context.Context, apiAppID int64) ([]APISecret, error) {
	if apiAppID <= 0 {
		return nil, NewValidationError("api app id must be greater than zero")
	}
	if _, err := s.repo.GetAPIAppByID(ctx, apiAppID); err != nil {
		return nil, err
	}
	return s.repo.ListAPISecrets(ctx, apiAppID)
}

func (s *service) BlockAPISecret(ctx context.Context, apiAppID int64, secretID string) (*APISecret, error) {
	secretID = strings.TrimSpace(secretID)
	if apiAppID <= 0 {
		return nil, NewValidationError("api app id must be greater than zero")
	}
	if secretID == "" {
		return nil, NewValidationError("secret id is required")
	}
	return s.repo.BlockAPISecret(ctx, apiAppID, secretID)
}

type cryptoRandomGenerator struct{}

func (cryptoRandomGenerator) Generate(prefix string, byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashAPISecret(secret string) string {
	hash := sha256.Sum256([]byte("stock-flow-api-secret\x00" + secret))
	return hex.EncodeToString(hash[:])
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return ListFilter{}, NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}
	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	}
	if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter, nil
}

func normalizeCreateAPIAppInput(input CreateAPIAppInput) (CreateAPIAppInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = normalizeOptionalText(input.Description)
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Metadata = normalizeMetadata(input.Metadata)
	if input.Status == "" {
		input.Status = StatusActive
	}
	if input.Name == "" {
		return CreateAPIAppInput{}, NewValidationError("name is required")
	}
	if !input.Status.IsValid() {
		return CreateAPIAppInput{}, NewValidationError("status must be active or inactive")
	}
	return input, nil
}

func normalizeUpdateAPIAppInput(input UpdateAPIAppInput) (UpdateAPIAppInput, error) {
	if input.ID <= 0 {
		return UpdateAPIAppInput{}, NewValidationError("api app id must be greater than zero")
	}
	if strings.TrimSpace(string(input.Status)) == "" {
		return UpdateAPIAppInput{}, NewValidationError("status is required")
	}
	normalized, err := normalizeCreateAPIAppInput(CreateAPIAppInput{
		Name: input.Name, Description: input.Description, Status: input.Status, Metadata: input.Metadata,
	})
	if err != nil {
		return UpdateAPIAppInput{}, err
	}
	input.Name = normalized.Name
	input.Description = normalized.Description
	input.Status = normalized.Status
	input.Metadata = normalized.Metadata
	return input, nil
}

func normalizeIssueSecretInput(input IssueSecretInput, now time.Time) (IssueSecretInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.BoundMetadata = normalizeMetadata(input.BoundMetadata)
	if input.APIAppID <= 0 {
		return IssueSecretInput{}, NewValidationError("api app id must be greater than zero")
	}
	if input.Name == "" {
		return IssueSecretInput{}, NewValidationError("name is required")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return IssueSecretInput{}, NewValidationError("expires_at must be in the future")
	}
	return input, nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeMetadata(metadata Metadata) Metadata {
	if metadata == nil {
		return Metadata{}
	}
	return cloneMetadata(metadata)
}

func cloneMetadata(metadata Metadata) Metadata {
	cloned := make(Metadata, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
