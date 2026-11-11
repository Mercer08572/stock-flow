package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceLoginVerifiesArgon2idPassword(t *testing.T) {
	hasher := testPasswordHasher()
	password := "admin-test-password"
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &fakeRepository{admin: &AdminUser{
		ID: 1, Username: "admin", PasswordHash: encoded, PasswordInitialized: true, Status: StatusActive,
	}}
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	service := NewService(repo, NewInMemorySessionStore(), hasher, ServiceOptions{
		Now:    func() time.Time { return now },
		Random: &sequenceRandom{values: []string{"session-token"}},
	})

	result, err := service.Login(context.Background(), LoginInput{Username: " admin ", Password: password})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.Admin.ID != 1 || result.Admin.Username != "admin" {
		t.Fatalf("unexpected admin identity: %+v", result.Admin)
	}
	if result.Token != "sess_session-token" {
		t.Fatalf("unexpected session token %q", result.Token)
	}

	_, err = service.Login(context.Background(), LoginInput{Username: "admin", Password: "incorrect"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestServiceRejectsExpiredSession(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySessionStore()
	if err := store.Save(context.Background(), Session{
		Token: "expired", AdminUserID: 1, Username: "admin", ExpiresAt: now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	service := NewService(&fakeRepository{}, store, testPasswordHasher(), ServiceOptions{
		Now: func() time.Time { return now },
	})

	_, err := service.AuthenticateAdminSession(context.Background(), "expired")
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected expired session, got %v", err)
	}
	if _, ok, err := store.Get(context.Background(), "expired"); err != nil || ok {
		t.Fatalf("expected expired session to be removed, ok=%v err=%v", ok, err)
	}
}

func TestServiceLogoutInvalidatesSession(t *testing.T) {
	store := NewInMemorySessionStore()
	if err := store.Save(context.Background(), Session{
		Token: "current-session", AdminUserID: 1, Username: "admin", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	service := NewService(&fakeRepository{}, store, testPasswordHasher(), ServiceOptions{})
	if err := service.Logout(context.Background(), "current-session"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.AuthenticateAdminSession(context.Background(), "current-session"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected invalidated session, got %v", err)
	}
}

func TestServiceRequiresInitializedPassword(t *testing.T) {
	hasher := testPasswordHasher()
	encoded, _ := hasher.Hash("admin-test-password")
	service := NewService(&fakeRepository{admin: &AdminUser{ID: 1, Username: "admin", PasswordHash: encoded, Status: StatusActive}}, NewInMemorySessionStore(), hasher, ServiceOptions{})
	_, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "admin-test-password", ClientIP: "127.0.0.1"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected uninitialized password rejection, got %v", err)
	}
}

func TestServiceInitializeAdminPasswordSucceedsOnceThenRefusesToOverwrite(t *testing.T) {
	hasher := testPasswordHasher()
	placeholder, err := hasher.Hash("placeholder-value-that-nobody-knows")
	if err != nil {
		t.Fatalf("hash placeholder: %v", err)
	}
	repo := &fakeRepository{admin: &AdminUser{
		ID: 1, Username: "admin", PasswordHash: placeholder, PasswordInitialized: false, MustChangePassword: true, Status: StatusActive,
	}}
	service := NewService(repo, NewInMemorySessionStore(), hasher, ServiceOptions{})
	ctx := context.Background()

	// The seeded row must stay unusable until an operator initializes it.
	if _, err := service.Login(ctx, LoginInput{Username: "admin", Password: "placeholder-value-that-nobody-knows", ClientIP: "127.0.0.1"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected the seeded placeholder to be unusable, got %v", err)
	}

	if err := service.InitializeAdminPassword(ctx, "admin", "first-deployment-password"); err != nil {
		t.Fatalf("initial initialize: %v", err)
	}
	if !repo.admin.PasswordInitialized || !repo.admin.MustChangePassword {
		t.Fatalf("expected initialized and forced-password-change state, got %+v", repo.admin)
	}
	firstHash := repo.admin.PasswordHash

	repo.admin.MustChangePassword = false
	err = service.InitializeAdminPassword(ctx, "admin", "second-deployment-password")
	if !errors.Is(err, ErrAdminAlreadyInitialized) {
		t.Fatalf("expected ErrAdminAlreadyInitialized on the second run, got %v", err)
	}
	if repo.admin.PasswordHash != firstHash {
		t.Fatal("expected the stored password hash to stay untouched after a refused re-initialization")
	}
	if !repo.admin.PasswordInitialized {
		t.Fatal("expected the administrator to stay initialized")
	}
}

func TestServiceChangePasswordInvalidatesOldSessionsAndIssuesNewSession(t *testing.T) {
	hasher := testPasswordHasher()
	encoded, _ := hasher.Hash("admin-test-password")
	repo := &fakeRepository{admin: &AdminUser{ID: 1, Username: "admin", PasswordHash: encoded, PasswordInitialized: true, MustChangePassword: true, Status: StatusActive}}
	store := NewInMemorySessionStore()
	service := NewService(repo, store, hasher, ServiceOptions{Random: &sequenceRandom{values: []string{"old", "new"}}})
	login, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "admin-test-password", ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	changed, err := service.ChangePassword(context.Background(), ChangePasswordInput{Token: login.Token, CurrentPassword: "admin-test-password", NewPassword: "a-new-secure-password"})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if changed.Admin.MustChangePassword {
		t.Fatal("expected forced-password-change state to be cleared")
	}
	if _, err := service.AuthenticateAdminSession(context.Background(), login.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected old session invalidated, got %v", err)
	}
	if _, err := service.AuthenticateAdminSession(context.Background(), changed.Token); err != nil {
		t.Fatalf("expected new session valid: %v", err)
	}
}

func TestServiceRateLimitsRepeatedLoginFailures(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{}, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{Now: func() time.Time { return now }, LoginFailurePolicy: RateLimitPolicy{MaxAttempts: 3, Window: 15 * time.Minute, Lockout: 15 * time.Minute}})
	for range 3 {
		_, _ = service.Login(context.Background(), LoginInput{Username: "admin", Password: "wrong", ClientIP: "127.0.0.1"})
	}
	_, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "wrong", ClientIP: "127.0.0.1"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestServiceIssuesAndAuthenticatesAPISecret(t *testing.T) {
	now := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	repo := &fakeRepository{app: &APIApp{
		ID: 5, AppID: "app_external", Name: "External", Status: StatusActive,
		Metadata: Metadata{"system": "orders"},
	}}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{
		Now:    func() time.Time { return now },
		Random: &sequenceRandom{values: []string{"secret-id", "plain-secret"}},
	})

	issued, err := service.IssueAPISecret(context.Background(), IssueSecretInput{
		APIAppID: 5,
		Name:     "primary",
		BoundMetadata: Metadata{
			"owner": "integration-team",
		},
	})
	if err != nil {
		t.Fatalf("issue secret: %v", err)
	}
	if issued.Secret != "sfsec_plain-secret" {
		t.Fatalf("unexpected plain secret %q", issued.Secret)
	}
	if repo.createdSecretHash == "" || repo.createdSecretHash == issued.Secret {
		t.Fatal("repository must receive only a one-way secret hash")
	}
	if issued.SecretHash != "" {
		t.Fatal("issued response must not expose the stored secret hash")
	}

	caller, err := service.AuthenticateAPIApp(context.Background(), "app_external", issued.Secret)
	if err != nil {
		t.Fatalf("authenticate api app: %v", err)
	}
	if caller.Type != CallerTypeAPIApp || caller.APIApp.SecretID != "key_secret-id" {
		t.Fatalf("unexpected caller: %+v", caller)
	}
	if caller.APIApp.BoundMetadata["owner"] != "integration-team" {
		t.Fatalf("missing bound metadata: %+v", caller.APIApp.BoundMetadata)
	}
	if !repo.touched {
		t.Fatal("expected successful authentication to update last_used_at")
	}
}

func TestServiceBlockedSecretCannotAuthenticate(t *testing.T) {
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_blocked", SecretHash: hashAPISecret("blocked-secret"), Status: SecretStatusBlocked,
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{})

	_, err := service.AuthenticateAPIApp(context.Background(), "app_external", "blocked-secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected blocked secret to fail, got %v", err)
	}
}

func TestServiceUpdateAPISecretKeepsMetadataAndStatus(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_primary", Name: "old-name",
			Status: SecretStatusActive, BoundMetadata: Metadata{"owner": "team-a"},
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{
		Now: func() time.Time { return now },
	})

	updated, err := service.UpdateAPISecret(context.Background(), UpdateSecretInput{
		APIAppID: 5, SecretID: "key_primary", Name: " new-name ", ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("update secret: %v", err)
	}
	if updated.Name != "new-name" {
		t.Fatalf("unexpected name %q", updated.Name)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expiry: %v", updated.ExpiresAt)
	}
	if updated.Status != SecretStatusActive {
		t.Fatalf("update must not change status, got %q", updated.Status)
	}
	if updated.BoundMetadata["owner"] != "team-a" {
		t.Fatalf("update must preserve bound metadata, got %+v", updated.BoundMetadata)
	}

	past := now.Add(-time.Hour)
	if _, err := service.UpdateAPISecret(context.Background(), UpdateSecretInput{
		APIAppID: 5, SecretID: "key_primary", Name: "new-name", ExpiresAt: &past,
	}); !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for past expiry, got %v", err)
	}
}

func TestServiceDeleteAPISecretInvalidatesIt(t *testing.T) {
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_primary", SecretHash: hashAPISecret("plain-secret"),
			Name: "primary", Status: SecretStatusActive, BoundMetadata: Metadata{},
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{})

	if err := service.DeleteAPISecret(context.Background(), 5, "key_primary"); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, err := service.AuthenticateAPIApp(context.Background(), "app_external", "plain-secret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected deleted secret to fail authentication, got %v", err)
	}
	if err := service.DeleteAPISecret(context.Background(), 5, "key_primary"); !errors.Is(err, ErrAPISecretNotFound) {
		t.Fatalf("expected second delete to return not found, got %v", err)
	}
}

func TestServiceUnblockAPISecretRestoresAuthentication(t *testing.T) {
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_primary", SecretHash: hashAPISecret("plain-secret"),
			Name: "primary", Status: SecretStatusBlocked, BoundMetadata: Metadata{},
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{})

	if _, err := service.AuthenticateAPIApp(context.Background(), "app_external", "plain-secret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected blocked secret to fail, got %v", err)
	}
	unblocked, err := service.UnblockAPISecret(context.Background(), 5, "key_primary")
	if err != nil {
		t.Fatalf("unblock secret: %v", err)
	}
	if unblocked.Status != SecretStatusActive {
		t.Fatalf("expected active status, got %q", unblocked.Status)
	}
	if _, err := service.AuthenticateAPIApp(context.Background(), "app_external", "plain-secret"); err != nil {
		t.Fatalf("expected unblocked secret to authenticate: %v", err)
	}
}

func TestServiceReissueAPISecretRotatesAndBlocksOldSecret(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_old", SecretHash: hashAPISecret("old-secret"),
			Name: "old", Status: SecretStatusActive, BoundMetadata: Metadata{"owner": "team-a"},
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{
		Now:    func() time.Time { return now },
		Random: &sequenceRandom{values: []string{"new-id", "new-secret", "new-id-2", "new-secret-2"}},
	})

	issued, err := service.ReissueAPISecret(context.Background(), ReissueSecretInput{
		APIAppID: 5, SecretID: "key_old", Name: "rotated", ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("reissue secret: %v", err)
	}
	if issued.Secret != "sfsec_new-secret" || issued.SecretID != "key_new-id" {
		t.Fatalf("unexpected issued secret: %+v", issued)
	}
	if issued.SecretHash != "" {
		t.Fatal("issued response must not expose the stored secret hash")
	}
	if issued.BoundMetadata["owner"] != "team-a" {
		t.Fatalf("replacement must inherit bound metadata, got %+v", issued.BoundMetadata)
	}

	if _, err := service.AuthenticateAPIApp(context.Background(), "app_external", "old-secret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old secret to be blocked, got %v", err)
	}
	if _, err := service.AuthenticateAPIApp(context.Background(), "app_external", issued.Secret); err != nil {
		t.Fatalf("expected new secret to authenticate: %v", err)
	}

	if _, err := service.ReissueAPISecret(context.Background(), ReissueSecretInput{
		APIAppID: 5, SecretID: "key_old", Name: "rotated-again",
	}); !errors.Is(err, ErrAPISecretNotActive) {
		t.Fatalf("expected old secret to be not active, got %v", err)
	}
}

func TestServiceReissueAPISecretValidatesInput(t *testing.T) {
	repo := &fakeRepository{app: &APIApp{ID: 5, AppID: "app_external", Status: StatusActive, Metadata: Metadata{}}}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{
		Random: &sequenceRandom{values: []string{"new-id", "new-secret"}},
	})

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := service.ReissueAPISecret(context.Background(), ReissueSecretInput{
		APIAppID: 5, SecretID: "key_old", Name: "rotated", ExpiresAt: &past,
	}); !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for past expiry, got %v", err)
	}
	if _, err := service.ReissueAPISecret(context.Background(), ReissueSecretInput{
		APIAppID: 5, SecretID: "key_old", Name: " ",
	}); !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for blank name, got %v", err)
	}
	if _, err := service.ReissueAPISecret(context.Background(), ReissueSecretInput{
		APIAppID: 6, SecretID: "key_old", Name: "rotated",
	}); !errors.Is(err, ErrAPIAppNotFound) {
		t.Fatalf("expected api app not found, got %v", err)
	}
}

func TestServiceInactiveAppCannotAuthenticate(t *testing.T) {
	repo := &fakeRepository{
		app: &APIApp{ID: 5, AppID: "app_external", Status: StatusInactive, Metadata: Metadata{}},
		secrets: []APISecret{{
			ID: 9, APIAppID: 5, SecretID: "key_active", SecretHash: hashAPISecret("active-secret"), Status: SecretStatusActive,
		}},
	}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{})

	_, err := service.AuthenticateAPIApp(context.Background(), "app_external", "active-secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected inactive app to fail, got %v", err)
	}
}

func testPasswordHasher() PasswordHasher {
	return NewArgon2idPasswordHasher(Argon2idParams{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16,
	})
}

type sequenceRandom struct {
	values []string
	index  int
}

func (r *sequenceRandom) Generate(prefix string, _ int) (string, error) {
	if r.index >= len(r.values) {
		return "", errors.New("no random value configured")
	}
	value := prefix + r.values[r.index]
	r.index++
	return value, nil
}

func TestServiceListAPIAppsReturnsTotal(t *testing.T) {
	repo := &fakeRepository{apps: []APIApp{
		{ID: 1, AppID: "app-1", Status: StatusActive},
		{ID: 2, AppID: "app-2", Status: StatusInactive},
	}}
	service := NewService(repo, NewInMemorySessionStore(), testPasswordHasher(), ServiceOptions{})

	result, err := service.ListAPIApps(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("list api apps: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 api apps, got %d", len(result.Items))
	}
}

type fakeRepository struct {
	admin             *AdminUser
	app               *APIApp
	apps              []APIApp
	secrets           []APISecret
	createdSecretHash string
	touched           bool
}

func (r *fakeRepository) GetAdminByID(_ context.Context, id int64) (*AdminUser, error) {
	if r.admin == nil || r.admin.ID != id {
		return nil, ErrAdminNotFound
	}
	copy := *r.admin
	return &copy, nil
}

func (r *fakeRepository) InitializeAdminPassword(_ context.Context, username string, passwordHash string) error {
	if r.admin == nil || r.admin.Username != username {
		return ErrAdminNotFound
	}
	if r.admin.PasswordInitialized {
		return ErrAdminAlreadyInitialized
	}
	r.admin.PasswordHash = passwordHash
	r.admin.PasswordInitialized = true
	r.admin.MustChangePassword = true
	return nil
}

func (r *fakeRepository) ChangeAdminPassword(_ context.Context, id int64, passwordHash string, changedAt time.Time) error {
	if r.admin == nil || r.admin.ID != id {
		return ErrAdminNotFound
	}
	r.admin.PasswordHash = passwordHash
	r.admin.MustChangePassword = false
	r.admin.PasswordChangedAt = &changedAt
	return nil
}

func (r *fakeRepository) GetAdminByUsername(_ context.Context, username string) (*AdminUser, error) {
	if r.admin == nil || r.admin.Username != username {
		return nil, ErrAdminNotFound
	}
	copy := *r.admin
	return &copy, nil
}

func (r *fakeRepository) ListAPIApps(context.Context, ListFilter) ([]APIApp, error) {
	return append([]APIApp(nil), r.apps...), nil
}

func (r *fakeRepository) CountAPIApps(context.Context, ListFilter) (int64, error) {
	return int64(len(r.apps)), nil
}

func (r *fakeRepository) GetAPIAppByID(_ context.Context, id int64) (*APIApp, error) {
	if r.app == nil || r.app.ID != id {
		return nil, ErrAPIAppNotFound
	}
	copy := *r.app
	return &copy, nil
}

func (r *fakeRepository) GetAPIAppByIdentifier(_ context.Context, appID string) (*APIApp, error) {
	if r.app == nil || r.app.AppID != appID {
		return nil, ErrAPIAppNotFound
	}
	copy := *r.app
	return &copy, nil
}

func (r *fakeRepository) CreateAPIApp(_ context.Context, appID string, input CreateAPIAppInput) (*APIApp, error) {
	r.app = &APIApp{ID: 1, AppID: appID, Name: input.Name, Description: input.Description, Status: input.Status, Metadata: input.Metadata}
	copy := *r.app
	return &copy, nil
}

func (r *fakeRepository) UpdateAPIApp(_ context.Context, input UpdateAPIAppInput) (*APIApp, error) {
	if r.app == nil || r.app.ID != input.ID {
		return nil, ErrAPIAppNotFound
	}
	r.app.Name = input.Name
	r.app.Description = input.Description
	r.app.Status = input.Status
	r.app.Metadata = input.Metadata
	copy := *r.app
	return &copy, nil
}

func (r *fakeRepository) SoftDeleteAPIApp(_ context.Context, id int64) error {
	if r.app == nil || r.app.ID != id {
		return ErrAPIAppNotFound
	}
	r.app = nil
	return nil
}

func (r *fakeRepository) CreateAPISecret(_ context.Context, secretID string, secretHash string, input IssueSecretInput) (*APISecret, error) {
	r.createdSecretHash = secretHash
	secret := APISecret{
		ID: 9, APIAppID: input.APIAppID, SecretID: secretID, Name: input.Name,
		Status: SecretStatusActive, BoundMetadata: input.BoundMetadata, ExpiresAt: input.ExpiresAt,
	}
	r.secrets = append(r.secrets, secret)
	return &secret, nil
}

func (r *fakeRepository) ListAPISecrets(context.Context, int64) ([]APISecret, error) {
	return append([]APISecret(nil), r.secrets...), nil
}

func (r *fakeRepository) ListAPISecretsForAuthentication(context.Context, int64) ([]APISecret, error) {
	secrets := append([]APISecret(nil), r.secrets...)
	for index := range secrets {
		if secrets[index].SecretHash == "" && secrets[index].ID == 9 {
			secrets[index].SecretHash = r.createdSecretHash
		}
	}
	return secrets, nil
}

func (r *fakeRepository) BlockAPISecret(_ context.Context, apiAppID int64, secretID string) (*APISecret, error) {
	for index := range r.secrets {
		if r.secrets[index].APIAppID == apiAppID && r.secrets[index].SecretID == secretID {
			r.secrets[index].Status = SecretStatusBlocked
			copy := r.secrets[index]
			return &copy, nil
		}
	}
	return nil, ErrAPISecretNotFound
}

func (r *fakeRepository) UpdateAPISecret(_ context.Context, input UpdateSecretInput) (*APISecret, error) {
	for index := range r.secrets {
		if r.secrets[index].APIAppID == input.APIAppID && r.secrets[index].SecretID == input.SecretID {
			r.secrets[index].Name = input.Name
			r.secrets[index].ExpiresAt = input.ExpiresAt
			copy := r.secrets[index]
			return &copy, nil
		}
	}
	return nil, ErrAPISecretNotFound
}

func (r *fakeRepository) SoftDeleteAPISecret(_ context.Context, apiAppID int64, secretID string) error {
	for index := range r.secrets {
		if r.secrets[index].APIAppID == apiAppID && r.secrets[index].SecretID == secretID {
			r.secrets = append(r.secrets[:index], r.secrets[index+1:]...)
			return nil
		}
	}
	return ErrAPISecretNotFound
}

func (r *fakeRepository) UnblockAPISecret(_ context.Context, apiAppID int64, secretID string) (*APISecret, error) {
	for index := range r.secrets {
		if r.secrets[index].APIAppID == apiAppID && r.secrets[index].SecretID == secretID {
			r.secrets[index].Status = SecretStatusActive
			copy := r.secrets[index]
			return &copy, nil
		}
	}
	return nil, ErrAPISecretNotFound
}

func (r *fakeRepository) ReissueAPISecret(_ context.Context, newSecretID string, newSecretHash string, input ReissueSecretInput) (*APISecret, error) {
	for index := range r.secrets {
		if r.secrets[index].APIAppID != input.APIAppID || r.secrets[index].SecretID != input.SecretID {
			continue
		}
		if r.secrets[index].Status != SecretStatusActive {
			return nil, ErrAPISecretNotActive
		}
		r.secrets[index].Status = SecretStatusBlocked
		r.createdSecretHash = newSecretHash
		secret := APISecret{
			ID: int64(len(r.secrets) + 10), APIAppID: input.APIAppID, SecretID: newSecretID,
			SecretHash: newSecretHash, Name: input.Name, Status: SecretStatusActive,
			BoundMetadata: r.secrets[index].BoundMetadata, ExpiresAt: input.ExpiresAt,
		}
		r.secrets = append(r.secrets, secret)
		response := secret
		response.SecretHash = ""
		return &response, nil
	}
	return nil, ErrAPISecretNotFound
}

func (r *fakeRepository) TouchAPISecretLastUsed(context.Context, int64, time.Time) error {
	r.touched = true
	return nil
}
