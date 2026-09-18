package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/shared/config"
)

const testDatabaseURL = "postgres://user:pass@localhost:5432/app?sslmode=disable"

func TestLoadAppliesDefaults(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("expected default environment development, got %q", cfg.Environment)
	}
	if cfg.GinMode != "debug" {
		t.Fatalf("expected default gin mode debug, got %q", cfg.GinMode)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default http addr :8080, got %q", cfg.HTTPAddr)
	}
	if cfg.ShutdownTimeout.String() != "10s" {
		t.Fatalf("expected default shutdown timeout 10s, got %s", cfg.ShutdownTimeout)
	}
	if cfg.EnvFile != "" {
		t.Fatalf("expected no env file to be loaded, got %q", cfg.EnvFile)
	}
}

func TestLoadDiscoversDefaultEnvFile(t *testing.T) {
	clearConfigEnv(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), `
APP_ENV=development
HTTP_ADDR=127.0.0.1:18080
DATABASE_URL=postgres://user:pass@localhost:5432/app?sslmode=disable
SHUTDOWN_TIMEOUT=15s
`)
	t.Chdir(dir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("expected http addr from .env, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected database url from .env, got %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout.String() != "15s" {
		t.Fatalf("expected 15s shutdown timeout, got %s", cfg.ShutdownTimeout)
	}
	if cfg.EnvFile != ".env" {
		t.Fatalf("expected default env file to be recorded, got %q", cfg.EnvFile)
	}
}

func TestLoadReadsExplicitEnvFile(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, `
APP_ENV=development
GIN_MODE=debug
HTTP_ADDR="127.0.0.1:18080"
DATABASE_URL="postgres://user:pass@localhost:5432/app?sslmode=disable"
SHUTDOWN_TIMEOUT="15s"
`)
	t.Setenv("ENV_FILE", envFile)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("expected development env, got %q", cfg.Environment)
	}
	if cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("expected http addr from env file, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected quoted database url to be unquoted, got %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout.String() != "15s" {
		t.Fatalf("expected 15s shutdown timeout, got %s", cfg.ShutdownTimeout)
	}
	if cfg.EnvFile != envFile {
		t.Fatalf("expected env file %q to be recorded, got %q", envFile, cfg.EnvFile)
	}
}

func TestExportedEnvOverridesEnvFile(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, `
APP_ENV=development
GIN_MODE=debug
HTTP_ADDR=:18080
DATABASE_URL=postgres://file:pass@localhost:5432/app?sslmode=disable
SHUTDOWN_TIMEOUT=15s
`)
	t.Setenv("ENV_FILE", envFile)
	t.Setenv("APP_ENV", "production")
	t.Setenv("GIN_MODE", "release")
	t.Setenv("DATABASE_URL", "postgres://exported:pass@localhost:5432/app?sslmode=disable")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("AUTH_ADMIN_COOKIE_SECURE", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "production" {
		t.Fatalf("expected exported APP_ENV to win, got %q", cfg.Environment)
	}
	if cfg.GinMode != "release" {
		t.Fatalf("expected exported GIN_MODE to win, got %q", cfg.GinMode)
	}
	if cfg.DatabaseURL != "postgres://exported:pass@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected exported DATABASE_URL to win, got %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout.String() != "30s" {
		t.Fatalf("expected exported SHUTDOWN_TIMEOUT 30s to win, got %s", cfg.ShutdownTimeout)
	}
	// Not exported, so the value still comes from the env file.
	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("expected http addr from env file, got %q", cfg.HTTPAddr)
	}
}

// A key present in the environment is applied even when blank: the blank wins over
// the env file and is NOT replaced by a built-in default. For HTTP_ADDR that means
// an explicit blank is a configuration error rather than a silent fallback.
func TestBlankExportedHTTPAddrFailsInsteadOfUsingDefaults(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, `
HTTP_ADDR=:18080
DATABASE_URL=postgres://file:pass@localhost:5432/app?sslmode=disable
`)
	t.Setenv("ENV_FILE", envFile)
	// Whitespace only: env() normalizes it to an empty value.
	t.Setenv("HTTP_ADDR", "   ")
	t.Setenv("DATABASE_URL", testDatabaseURL)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an explicit blank HTTP_ADDR to fail validation")
	}
	if !strings.Contains(err.Error(), "HTTP_ADDR") {
		t.Fatalf("expected the error to name HTTP_ADDR, got %v", err)
	}
}

// A blank value is applied to the config instead of being replaced by the built-in
// default. APP_ENV has a default, so the failure proves the blank was applied.
func TestBlankExportedAPPEnvIsAppliedAndFailsValidation(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("APP_ENV", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected a blank APP_ENV to fail validation instead of using the default")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("expected the error to name APP_ENV, got %v", err)
	}
}

// Numeric, duration, and boolean fields reject an explicit blank as a parse error
// rather than quietly using the default.
func TestBlankExportedNonStringValuesFailParsing(t *testing.T) {
	for _, key := range []string{
		"SHUTDOWN_TIMEOUT",
		"AUTH_ADMIN_SESSION_TTL",
		"AUTH_ADMIN_COOKIE_SECURE",
		"AUTH_LOGIN_FAILURE_MAX_ATTEMPTS",
	} {
		t.Run(key, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(key, "")

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected a blank %s to be rejected", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("expected the error to name %s, got %v", key, err)
			}
		})
	}
}

// A blank value written in the env file is applied the same way as a blank export.
func TestBlankEnvFileValueIsAppliedNotDefaulted(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, `
APP_ENV=
DATABASE_URL=postgres://file:pass@localhost:5432/app?sslmode=disable
`)
	t.Setenv("ENV_FILE", envFile)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected a blank APP_ENV from the env file to fail validation")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("expected the error to name APP_ENV, got %v", err)
	}
}

func TestLoadWithoutEnvFileSucceeds(t *testing.T) {
	clearConfigEnv(t)

	// ENV_FILE is unset, so the optional .env lookup happens in the package
	// directory, where no .env file exists.
	t.Setenv("DATABASE_URL", testDatabaseURL)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected a missing .env to be tolerated, got %v", err)
	}
	if cfg.EnvFile != "" {
		t.Fatalf("expected no env file to be recorded, got %q", cfg.EnvFile)
	}
}

func TestLoadRequiresExplicitEnvFile(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))

	if _, err := config.Load(); err == nil {
		t.Fatal("expected missing explicit env file error")
	}
}

func TestLoadRejectsMalformedEnvFile(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, "BAD@KEY=value\n")
	t.Setenv("ENV_FILE", envFile)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected malformed env file error")
	}
	if !strings.Contains(err.Error(), "read env file") {
		t.Fatalf("expected env file context in error, got %v", err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	clearConfigEnv(t)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected missing DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected DATABASE_URL hint, got %v", err)
	}
}

func TestLoadNormalizesPortAlias(t *testing.T) {
	clearConfigEnv(t)

	envFile := writeEnvFile(t, "DATABASE_URL=postgres://user:pass@localhost:5432/app?sslmode=disable\n")
	t.Setenv("ENV_FILE", envFile)
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected PORT to normalize to :9090, got %q", cfg.HTTPAddr)
	}
}

func TestLoadRejectsHTTPAddrWithoutPort(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("HTTP_ADDR", "127.0.0.1")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected invalid http addr error")
	}
}

// clearConfigEnv removes every supported key. godotenv.Load keeps any key that is
// already present in the environment, so clearing must unset the variables rather
// than blank them. Original values are restored when the test finishes.
func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"APP_ENV",
		"ENV_FILE",
		"GIN_MODE",
		"HTTP_ADDR",
		"PORT",
		"DATABASE_URL",
		"SHUTDOWN_TIMEOUT",
		"AUTH_ADMIN_SESSION_TTL",
		"AUTH_ADMIN_COOKIE_SAME_SITE",
		"AUTH_ADMIN_COOKIE_SECURE",
		"AUTH_LOGIN_FAILURE_MAX_ATTEMPTS",
		"AUTH_LOGIN_FAILURE_WINDOW",
		"AUTH_LOGIN_LOCKOUT",
		"AUTH_LOGIN_IP_MAX_ATTEMPTS",
	}

	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { _ = os.Setenv(key, value) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}
		_ = os.Unsetenv(key)
	}
}

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, content)

	return path
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
