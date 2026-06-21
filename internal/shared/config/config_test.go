package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/shared/config"
)

func TestLoadUsesConfigFile(t *testing.T) {
	clearConfigEnv(t)

	configFile := writeConfigFile(t, `
app_env: development
gin_mode: debug
http_addr: "127.0.0.1:18080"
database_url: "postgres://user:pass@localhost:5432/app?sslmode=disable"
shutdown_timeout: "15s"
`)
	t.Setenv("CONFIG_FILE", configFile)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("expected development env, got %q", cfg.Environment)
	}
	if cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("expected config http addr, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected database url from config file, got %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout.String() != "15s" {
		t.Fatalf("expected 15s shutdown timeout, got %s", cfg.ShutdownTimeout)
	}
	if cfg.ConfigFile != configFile {
		t.Fatalf("expected config file path to be recorded")
	}
}

func TestLoadEnvOverridesConfigFile(t *testing.T) {
	clearConfigEnv(t)

	configFile := writeConfigFile(t, `
app_env: development
gin_mode: debug
http_addr: ":8080"
database_url: "postgres://user:pass@localhost:5432/app?sslmode=disable"
shutdown_timeout: "15s"
`)
	t.Setenv("CONFIG_FILE", configFile)
	t.Setenv("APP_ENV", "production")
	t.Setenv("GIN_MODE", "release")
	t.Setenv("DATABASE_URL", "postgres://prod:pass@localhost:5432/app?sslmode=disable")
	t.Setenv("PORT", ":9090")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "production" {
		t.Fatalf("expected env override production, got %q", cfg.Environment)
	}
	if cfg.GinMode != "release" {
		t.Fatalf("expected env override release, got %q", cfg.GinMode)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected normalized port override :9090, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://prod:pass@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected env database url override, got %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout.String() != "30s" {
		t.Fatalf("expected env timeout override 30s, got %s", cfg.ShutdownTimeout)
	}
}

func TestLoadRequiresExplicitConfigFile(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected missing explicit config file error")
	}
}

func TestLoadIgnoresMissingImplicitConfigFile(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("APP_ENV", "local-test-without-file")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/app?sslmode=disable")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "local-test-without-file" {
		t.Fatalf("expected APP_ENV value, got %q", cfg.Environment)
	}
	if cfg.ConfigFile != "" {
		t.Fatalf("expected no config file to be recorded, got %q", cfg.ConfigFile)
	}
}

func TestLoadRejectsHTTPAddrWithoutPort(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/app?sslmode=disable")
	t.Setenv("HTTP_ADDR", "127.0.0.1")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected invalid http addr error")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"APP_ENV",
		"CONFIG_FILE",
		"GIN_MODE",
		"HTTP_ADDR",
		"PORT",
		"DATABASE_URL",
		"SHUTDOWN_TIMEOUT",
	}

	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}
