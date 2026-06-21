package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	Environment     string
	GinMode         string
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	ConfigFile      string
}

type fileConfig struct {
	Environment     string `yaml:"app_env"`
	GinMode         string `yaml:"gin_mode"`
	HTTPAddr        string `yaml:"http_addr"`
	DatabaseURL     string `yaml:"database_url"`
	ShutdownTimeout string `yaml:"shutdown_timeout"`
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
		Environment:     defaultEnvironment,
		GinMode:         defaultGinMode,
		HTTPAddr:        defaultHTTPAddr,
		ShutdownTimeout: defaultShutdownTimeout,
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
	if value := env(configFileEnv); value != "" {
		cfg.ConfigFile = value
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
