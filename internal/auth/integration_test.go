//go:build integration

package auth_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mercer08572/stock-flow/internal/auth"
)

// TestAdminBootstrapAgainstMigratedDatabase walks the deployment bootstrap path against a real
// PostgreSQL database: apply every migration in migrations/, then initialize the administrator.
//
// It only runs when TEST_DATABASE_URL points at a disposable database, because it drops and
// recreates the whole schema. TestDatabaseURL is an environment variable, so unlike
// internal/shared/config.Load it never falls back to .env or to DATABASE_URL.
func TestAdminBootstrapAgainstMigratedDatabase(t *testing.T) {
	requireIntegrationEnv(t)

	pool := newTestPool(t)
	runMigrate(t, "drop", "-f")
	runMigrate(t, "up")

	var adminRows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM admin_users`).Scan(&adminRows); err != nil {
		t.Fatalf("count seeded administrators: %v", err)
	}
	if adminRows != 1 {
		t.Fatalf("expected exactly one seeded administrator, got %d", adminRows)
	}

	var username, passwordHash string
	var passwordInitialized, mustChangePassword bool
	err := pool.QueryRow(context.Background(),
		`SELECT username, password_hash, password_initialized, must_change_password
		 FROM admin_users
		 WHERE deleted_at IS NULL`).
		Scan(&username, &passwordHash, &passwordInitialized, &mustChangePassword)
	if err != nil {
		t.Fatalf("read seeded administrator: %v", err)
	}
	if username != "admin" {
		t.Fatalf("expected the seed to use the username %q, got %q", "admin", username)
	}
	if passwordInitialized {
		t.Fatal("the seeded administrator must not be marked as initialized")
	}
	if !mustChangePassword {
		t.Fatal("the seeded administrator must require a password change")
	}

	hasher := auth.NewArgon2idPasswordHasher(auth.Argon2idParams{Memory: 64, Iterations: 1, Parallelism: 1})
	if ok, err := hasher.Verify("anything-an-operator-might-type", passwordHash); err != nil {
		t.Fatalf("verify seeded placeholder hash: %v", err)
	} else if ok {
		t.Fatal("the seeded placeholder hash must not be verifiable by an arbitrary password")
	}

	service := auth.NewService(
		auth.NewPostgresRepository(pool),
		auth.NewInMemorySessionStore(),
		hasher,
		auth.ServiceOptions{},
	)
	ctx := context.Background()

	// Login with the placeholder must fail while password_initialized is FALSE.
	_, err = service.Login(ctx, auth.LoginInput{Username: username, Password: "anything-an-operator-might-type", ClientIP: "127.0.0.1"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for the uninitialized administrator, got %v", err)
	}

	const initialPassword = "deployment-initial-password"
	if err := service.InitializeAdminPassword(ctx, username, initialPassword); err != nil {
		t.Fatalf("initialize administrator password: %v", err)
	}

	storedHash, storedInitialized, storedMustChange := readAdminState(t, pool)
	if !storedInitialized {
		t.Fatal("expected password_initialized to be TRUE after initialization")
	}
	if !storedMustChange {
		t.Fatal("expected must_change_password to stay TRUE after initialization")
	}
	if ok, err := hasher.Verify(initialPassword, storedHash); err != nil {
		t.Fatalf("verify stored hash: %v", err)
	} else if !ok {
		t.Fatal("expected the initialized password to verify against the stored hash")
	}

	// The same command must not overwrite an already initialized administrator.
	err = service.InitializeAdminPassword(ctx, username, "second-deployment-password")
	if !errors.Is(err, auth.ErrAdminAlreadyInitialized) {
		t.Fatalf("expected ErrAdminAlreadyInitialized on the second run, got %v", err)
	}
	storedHashAfter, _, _ := readAdminState(t, pool)
	if storedHashAfter != storedHash {
		t.Fatal("expected the stored password hash to stay untouched after a refused re-initialization")
	}

	// A missing administrator keeps its dedicated error so the CLI can tell operators what to do.
	if err := service.InitializeAdminPassword(ctx, "does-not-exist", initialPassword); !errors.Is(err, auth.ErrAdminNotFound) {
		t.Fatalf("expected ErrAdminNotFound for an unknown username, got %v", err)
	}

	verifyMigrationMetadataIsApplied(t)
}

func requireIntegrationEnv(t *testing.T) {
	t.Helper()
	if os.Getenv(testDatabaseURLEnv) == "" {
		t.Skipf("set %s to a disposable database to run the integration tests", testDatabaseURLEnv)
	}
	if resolveMigrateBinary() == "" {
		t.Skip("the golang-migrate CLI is required to run the integration tests")
	}
}

const testDatabaseURLEnv = "TEST_DATABASE_URL"

// newTestPool connects to the disposable database described by TEST_DATABASE_URL while keeping
// the developer .env file out of the picture.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// config.Load reads .env from the process working directory; pin it to an empty file so the
	// database under test is decided solely by TEST_DATABASE_URL.
	envFile := filepath.Join(t.TempDir(), "empty.env")
	if err := os.WriteFile(envFile, nil, 0o600); err != nil {
		t.Fatalf("write isolated env file: %v", err)
	}
	t.Setenv("ENV_FILE", envFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv(testDatabaseURLEnv))
	if err != nil {
		t.Fatalf("connect to %s: %v", testDatabaseURLEnv, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping %s: %v", testDatabaseURLEnv, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func runMigrate(t *testing.T, args ...string) {
	t.Helper()
	migrationsPath, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations directory: %v", err)
	}
	commandArgs := append([]string{
		"-path", migrationsPath,
		"-database", migrateDatabaseURL(),
	}, args...)
	output, err := exec.Command(resolveMigrateBinary(), commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("migrate %v: %v\n%s", args, err, output)
	}
}

func migrateDatabaseURL() string {
	return "pgx5://" + strings.TrimPrefix(strings.TrimPrefix(os.Getenv(testDatabaseURLEnv), "postgres://"), "postgresql://")
}

func resolveMigrateBinary() string {
	if path, err := exec.LookPath("migrate"); err == nil {
		return path
	}
	return ""
}

func readAdminState(t *testing.T, pool *pgxpool.Pool) (passwordHash string, initialized bool, mustChange bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT password_hash, password_initialized, must_change_password
		 FROM admin_users
		 WHERE username = 'admin' AND deleted_at IS NULL`).
		Scan(&passwordHash, &initialized, &mustChange)
	if err != nil {
		t.Fatalf("read administrator state: %v", err)
	}
	return passwordHash, initialized, mustChange
}

// verifyMigrationMetadataIsApplied guards the migrations/AGENTS.md rule that every migration
// already recorded in the database must be marked as applied in its own file.
func verifyMigrationMetadataIsApplied(t *testing.T) {
	t.Helper()
	migrationsPath, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations directory: %v", err)
	}
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(migrationsPath, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if !strings.Contains(string(content), "-- status: applied") {
			t.Fatalf("%s is applied in the database but still declares another status", entry.Name())
		}
	}
}
