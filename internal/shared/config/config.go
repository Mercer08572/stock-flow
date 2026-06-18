package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultEnvironment     = "development"
	defaultGinMode         = "debug"
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second
)

// Config contains process-level settings loaded at application startup.
type Config struct {
	Environment     string
	GinMode         string
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:     env("APP_ENV", defaultEnvironment),
		GinMode:         env("GIN_MODE", defaultGinMode),
		HTTPAddr:        httpAddr(),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if err := validateGinMode(cfg.GinMode); err != nil {
		return Config{}, err
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = timeout
	}

	return cfg, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func httpAddr() string {
	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		return addr
	}

	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return ":" + port
	}

	return defaultHTTPAddr
}

func validateGinMode(mode string) error {
	switch mode {
	case "debug", "release", "test":
		return nil
	default:
		return fmt.Errorf("GIN_MODE must be one of debug, release, or test")
	}
}
