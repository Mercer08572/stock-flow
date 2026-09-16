package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/auth"
)

func TestRunInitInitializesAdministrator(t *testing.T) {
	t.Setenv(initialPasswordEnv, "")
	service := &fakeAdminService{}
	factory := func(context.Context) (auth.Service, func(), error) {
		return service, func() {}, nil
	}
	stdout := &bytes.Buffer{}

	if err := runWithDeps([]string{"init", "--password-stdin"}, strings.NewReader("deployment-password\n"), stdout, factory); err != nil {
		t.Fatalf("run init: %v", err)
	}
	if service.username != "admin" || service.password != "deployment-password" {
		t.Fatalf("unexpected initialization input: username=%q password=%q", service.username, service.password)
	}
	if !strings.Contains(stdout.String(), `administrator "admin" initialized`) {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
}

func TestRunInitRefusesToOverwriteInitializedAdministrator(t *testing.T) {
	t.Setenv(initialPasswordEnv, "")
	service := &fakeAdminService{err: auth.ErrAdminAlreadyInitialized}
	factory := func(context.Context) (auth.Service, func(), error) {
		return service, func() {}, nil
	}
	stdout := &bytes.Buffer{}

	err := runWithDeps([]string{"init", "--password-stdin"}, strings.NewReader("deployment-password\n"), stdout, factory)
	if err == nil {
		t.Fatal("expected the second initialization to fail")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Fatalf("expected an explicit already-initialized error, got %v", err)
	}
	if strings.Contains(err.Error(), "deployment-password") {
		t.Fatalf("error message must not echo the password: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no success output, got %q", stdout.String())
	}
}

func TestRunInitReportsMissingAdministratorRow(t *testing.T) {
	t.Setenv(initialPasswordEnv, "")
	factory := func(context.Context) (auth.Service, func(), error) {
		return &fakeAdminService{err: auth.ErrAdminNotFound}, func() {}, nil
	}

	err := runWithDeps([]string{"init", "--password-stdin"}, strings.NewReader("deployment-password\n"), &bytes.Buffer{}, factory)
	if err == nil || !strings.Contains(err.Error(), "run the database migrations") {
		t.Fatalf("expected actionable migration guidance, got %v", err)
	}
}

func TestRunInitRejectsUnknownCommand(t *testing.T) {
	factory := func(context.Context) (auth.Service, func(), error) {
		t.Fatal("the service factory must not be called for an invalid command")
		return nil, nil, nil
	}

	if err := runWithDeps([]string{"bootstrap"}, strings.NewReader(""), &bytes.Buffer{}, factory); err == nil {
		t.Fatal("expected a usage error")
	}
}

func TestReadInitialPasswordPrefersEnvironmentVariable(t *testing.T) {
	t.Setenv(initialPasswordEnv, "from-environment")

	password, err := readInitialPassword(true, strings.NewReader("from-stdin\n"))
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	if password != "from-environment" {
		t.Fatalf("unexpected password %q", password)
	}
}

func TestDescribeInitializeErrorKeepsUnexpectedErrors(t *testing.T) {
	cause := errors.New("connection refused")

	if err := describeInitializeError("admin", cause); !errors.Is(err, cause) {
		t.Fatalf("expected the original error to be preserved, got %v", err)
	}
}

type fakeAdminService struct {
	username string
	password string
	err      error
}

func (s *fakeAdminService) AuthenticateAdminSession(context.Context, string) (auth.Caller, error) {
	return auth.Caller{}, auth.ErrUnauthorized
}

func (s *fakeAdminService) AuthenticateAPIApp(context.Context, string, string) (auth.Caller, error) {
	return auth.Caller{}, auth.ErrUnauthorized
}

func (s *fakeAdminService) Login(context.Context, auth.LoginInput) (auth.LoginResult, error) {
	return auth.LoginResult{}, auth.ErrInvalidCredentials
}

func (s *fakeAdminService) Logout(context.Context, string) error { return nil }

func (s *fakeAdminService) Me(context.Context, string) (auth.AdminIdentity, error) {
	return auth.AdminIdentity{}, auth.ErrUnauthorized
}

func (s *fakeAdminService) ChangePassword(context.Context, auth.ChangePasswordInput) (auth.LoginResult, error) {
	return auth.LoginResult{}, auth.ErrUnauthorized
}

func (s *fakeAdminService) InitializeAdminPassword(_ context.Context, username string, password string) error {
	s.username = username
	s.password = password
	return s.err
}

func (s *fakeAdminService) ListAPIApps(context.Context, auth.ListFilter) (auth.APIAppListResult, error) {
	return auth.APIAppListResult{}, nil
}

func (s *fakeAdminService) GetAPIApp(context.Context, int64) (*auth.APIApp, error) {
	return nil, auth.ErrAPIAppNotFound
}

func (s *fakeAdminService) CreateAPIApp(context.Context, auth.CreateAPIAppInput) (*auth.APIApp, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeAdminService) UpdateAPIApp(context.Context, auth.UpdateAPIAppInput) (*auth.APIApp, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeAdminService) DeleteAPIApp(context.Context, int64) error { return nil }

func (s *fakeAdminService) IssueAPISecret(context.Context, auth.IssueSecretInput) (*auth.IssuedSecret, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeAdminService) ListAPISecrets(context.Context, int64) ([]auth.APISecret, error) {
	return nil, nil
}

func (s *fakeAdminService) BlockAPISecret(context.Context, int64, string) (*auth.APISecret, error) {
	return nil, errors.New("not implemented")
}

var _ auth.Service = (*fakeAdminService)(nil)
