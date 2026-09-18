package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultEnvironment     = "development"
	defaultGinMode         = "debug"
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second

	// defaultEnvFile is read from the process working directory.
	defaultEnvFile = ".env"
	// envFileEnv optionally points at a different dotenv file, for example when
	// the process does not start from the project root.
	envFileEnv = "ENV_FILE"
)

// Config contains process-level settings loaded at application startup.
type Config struct {
	Environment                 string
	GinMode                     string
	HTTPAddr                    string
	DatabaseURL                 string
	ShutdownTimeout             time.Duration
	AuthAdminSessionTTL         time.Duration
	AuthAdminCookieSameSite     string
	AuthAdminCookieSecure       bool
	AuthLoginFailureMaxAttempts int
	AuthLoginFailureWindow      time.Duration
	AuthLoginLockout            time.Duration
	AuthLoginIPMaxAttempts      int
	// EnvFile is the dotenv file that was loaded, or empty when none was found.
	EnvFile string
}

// Load resolves configuration using the precedence:
//
//	exported environment variables > .env file > built-in defaults
//
// A variable counts as exported when it is present in the environment, even with
// a blank value, so a blank export keeps the .env file from filling it in. The
// .env file itself is optional because deployments are expected to inject real
// environment variables; a missing file is an error only when ENV_FILE asked
// for it explicitly.
func Load() (Config, error) {
	envFile, err := loadEnvFile()
	if err != nil {
		return Config{}, err
	}

	cfg := defaultConfig()
	cfg.EnvFile = envFile

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Environment:                 defaultEnvironment,
		GinMode:                     defaultGinMode,
		HTTPAddr:                    defaultHTTPAddr,
		ShutdownTimeout:             defaultShutdownTimeout,
		AuthAdminSessionTTL:         8 * time.Hour,
		AuthAdminCookieSameSite:     "lax",
		AuthLoginFailureMaxAttempts: 5,
		AuthLoginFailureWindow:      15 * time.Minute,
		AuthLoginLockout:            15 * time.Minute,
		AuthLoginIPMaxAttempts:      30,
	}
}

// loadEnvFile copies the dotenv file into the process environment and returns
// the path it read. godotenv.Load only fills keys that are absent from the
// environment, so an exported value always wins -- including a blank one: use
// `unset FOO` rather than `export FOO=` when a value must come from the file.
func loadEnvFile() (string, error) {
	path := env(envFileEnv)
	explicit := path != ""
	if !explicit {
		path = defaultEnvFile
	}

	err := godotenv.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			// No dotenv file is the normal case in production and CI.
			return "", nil
		}
		return "", fmt.Errorf("read env file %q: %w", path, err)
	}

	return path, nil
}

func applyEnv(cfg *Config) error {
	if value := env("APP_ENV"); value != "" {
		cfg.Environment = value
	}
	if value := env("GIN_MODE"); value != "" {
		cfg.GinMode = value
	}
	if value := env("HTTP_ADDR"); value != "" {
		cfg.HTTPAddr = value
	} else if value := env("PORT"); value != "" {
		cfg.HTTPAddr = addressFromPort(value)
	}
	if value := env("DATABASE_URL"); value != "" {
		cfg.DatabaseURL = value
	}
	if value := env("SHUTDOWN_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = timeout
	}
	if err := applyAuthEnv(cfg); err != nil {
		return err
	}

	return nil
}

func applyAuthEnv(cfg *Config) error {
	durations := []struct {
		key    string
		target *time.Duration
	}{{"AUTH_ADMIN_SESSION_TTL", &cfg.AuthAdminSessionTTL}, {"AUTH_LOGIN_FAILURE_WINDOW", &cfg.AuthLoginFailureWindow}, {"AUTH_LOGIN_LOCKOUT", &cfg.AuthLoginLockout}}
	for _, item := range durations {
		if value := env(item.key); value != "" {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("parse %s: %w", item.key, err)
			}
			*item.target = parsed
		}
	}
	if value := strings.ToLower(env("AUTH_ADMIN_COOKIE_SAME_SITE")); value != "" {
		cfg.AuthAdminCookieSameSite = value
	}
	if value := env("AUTH_ADMIN_COOKIE_SECURE"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse AUTH_ADMIN_COOKIE_SECURE: %w", err)
		}
		cfg.AuthAdminCookieSecure = parsed
	}
	ints := []struct {
		key    string
		target *int
	}{{"AUTH_LOGIN_FAILURE_MAX_ATTEMPTS", &cfg.AuthLoginFailureMaxAttempts}, {"AUTH_LOGIN_IP_MAX_ATTEMPTS", &cfg.AuthLoginIPMaxAttempts}}
	for _, item := range ints {
		if value := env(item.key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("parse %s: %w", item.key, err)
			}
			*item.target = parsed
		}
	}
	return nil
}

// env returns the value of key with surrounding whitespace removed. It normalizes
// configuration values only; it no longer decides precedence, which now belongs to
// godotenv.Load (a key is either present in the environment or it is not).
func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func addressFromPort(port string) string {
	port = strings.TrimSpace(port)
	if strings.HasPrefix(port, ":") {
		return port
	}

	return ":" + port
}

func validate(cfg Config) error {
	if cfg.Environment == "" {
		return errors.New("APP_ENV is required")
	}
	if err := validateGinMode(cfg.GinMode); err != nil {
		return err
	}
	if err := validateHTTPAddr(cfg.HTTPAddr); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if cfg.AuthAdminSessionTTL < 15*time.Minute || cfg.AuthAdminSessionTTL > 24*time.Hour {
		return errors.New("AUTH_ADMIN_SESSION_TTL must be between 15m and 24h")
	}
	if cfg.AuthAdminCookieSameSite != "lax" && cfg.AuthAdminCookieSameSite != "strict" && cfg.AuthAdminCookieSameSite != "none" {
		return errors.New("AUTH_ADMIN_COOKIE_SAME_SITE must be lax, strict, or none")
	}
	if cfg.AuthAdminCookieSameSite == "none" && !cfg.AuthAdminCookieSecure {
		return errors.New("AUTH_ADMIN_COOKIE_SECURE must be true when SameSite is none")
	}
	if cfg.Environment == "production" && !cfg.AuthAdminCookieSecure {
		return errors.New("AUTH_ADMIN_COOKIE_SECURE must be true in production")
	}
	if cfg.AuthLoginFailureMaxAttempts < 3 || cfg.AuthLoginFailureMaxAttempts > 20 {
		return errors.New("AUTH_LOGIN_FAILURE_MAX_ATTEMPTS must be between 3 and 20")
	}
	if cfg.AuthLoginIPMaxAttempts < cfg.AuthLoginFailureMaxAttempts {
		return errors.New("AUTH_LOGIN_IP_MAX_ATTEMPTS must not be less than the failure limit")
	}
	if cfg.AuthLoginFailureWindow < time.Minute || cfg.AuthLoginFailureWindow > time.Hour {
		return errors.New("AUTH_LOGIN_FAILURE_WINDOW must be between 1m and 1h")
	}
	if cfg.AuthLoginLockout < time.Minute || cfg.AuthLoginLockout > time.Hour {
		return errors.New("AUTH_LOGIN_LOCKOUT must be between 1m and 1h")
	}

	return nil
}

func validateGinMode(mode string) error {
	switch mode {
	case "debug", "release", "test":
		return nil
	default:
		return fmt.Errorf("GIN_MODE must be one of debug, release, or test")
	}
}

func validateHTTPAddr(addr string) error {
	if addr == "" {
		return errors.New("HTTP_ADDR is required")
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("HTTP_ADDR must include a host and port, such as :8080 or 127.0.0.1:8080")
	}
	if port == "" {
		return errors.New("HTTP_ADDR port is required")
	}

	return nil
}
