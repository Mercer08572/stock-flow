package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultEnvironment     = "development"
	defaultGinMode         = "debug"
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second

	configFileEnv = "CONFIG_FILE"
	configDir     = "configs"
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
	ConfigFile                  string
}

type fileConfig struct {
	Environment                 string `yaml:"app_env"`
	GinMode                     string `yaml:"gin_mode"`
	HTTPAddr                    string `yaml:"http_addr"`
	DatabaseURL                 string `yaml:"database_url"`
	ShutdownTimeout             string `yaml:"shutdown_timeout"`
	AuthAdminSessionTTL         string `yaml:"auth_admin_session_ttl"`
	AuthAdminCookieSameSite     string `yaml:"auth_admin_cookie_same_site"`
	AuthAdminCookieSecure       *bool  `yaml:"auth_admin_cookie_secure"`
	AuthLoginFailureMaxAttempts int    `yaml:"auth_login_failure_max_attempts"`
	AuthLoginFailureWindow      string `yaml:"auth_login_failure_window"`
	AuthLoginLockout            string `yaml:"auth_login_lockout"`
	AuthLoginIPMaxAttempts      int    `yaml:"auth_login_ip_max_attempts"`
}

func Load() (Config, error) {
	cfg := defaultConfig()

	if value := env("APP_ENV"); value != "" {
		cfg.Environment = value
	}

	configFile, explicitConfigFile := resolveConfigFile(cfg.Environment)
	if configFile != "" {
		if err := applyConfigFile(&cfg, configFile, explicitConfigFile); err != nil {
			return Config{}, err
		}
	}

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

func resolveConfigFile(environment string) (string, bool) {
	if configFile := env(configFileEnv); configFile != "" {
		return configFile, true
	}

	if environment == "" {
		environment = defaultEnvironment
	}

	return filepath.Join(configDir, environment+".yaml"), false
}

func applyConfigFile(cfg *Config, path string, required bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return nil
		}
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	var fileCfg fileConfig
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	if value := strings.TrimSpace(fileCfg.Environment); value != "" {
		cfg.Environment = value
	}
	if value := strings.TrimSpace(fileCfg.GinMode); value != "" {
		cfg.GinMode = value
	}
	if value := strings.TrimSpace(fileCfg.HTTPAddr); value != "" {
		cfg.HTTPAddr = value
	}
	if value := strings.TrimSpace(fileCfg.DatabaseURL); value != "" {
		cfg.DatabaseURL = value
	}
	if value := strings.TrimSpace(fileCfg.ShutdownTimeout); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse shutdown_timeout in %q: %w", path, err)
		}
		cfg.ShutdownTimeout = timeout
	}
	if value := strings.TrimSpace(fileCfg.AuthAdminSessionTTL); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse auth_admin_session_ttl in %q: %w", path, err)
		}
		cfg.AuthAdminSessionTTL = duration
	}
	if value := strings.ToLower(strings.TrimSpace(fileCfg.AuthAdminCookieSameSite)); value != "" {
		cfg.AuthAdminCookieSameSite = value
	}
	if fileCfg.AuthAdminCookieSecure != nil {
		cfg.AuthAdminCookieSecure = *fileCfg.AuthAdminCookieSecure
	}
	if fileCfg.AuthLoginFailureMaxAttempts != 0 {
		cfg.AuthLoginFailureMaxAttempts = fileCfg.AuthLoginFailureMaxAttempts
	}
	if value := strings.TrimSpace(fileCfg.AuthLoginFailureWindow); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse auth_login_failure_window in %q: %w", path, err)
		}
		cfg.AuthLoginFailureWindow = duration
	}
	if value := strings.TrimSpace(fileCfg.AuthLoginLockout); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse auth_login_lockout in %q: %w", path, err)
		}
		cfg.AuthLoginLockout = duration
	}
	if fileCfg.AuthLoginIPMaxAttempts != 0 {
		cfg.AuthLoginIPMaxAttempts = fileCfg.AuthLoginIPMaxAttempts
	}

	cfg.ConfigFile = path
	return nil
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
	if value := env(configFileEnv); value != "" {
		cfg.ConfigFile = value
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
