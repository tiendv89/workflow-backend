// Package configs provides viper-backed YAML config loading with env-variable overrides.
package configs

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for api-service.
type Config struct {
	Log      LogConfig
	API      APIConfig
	Database DatabaseConfig
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string
}

// APIConfig holds HTTP server settings.
type APIConfig struct {
	Port                  int
	StaleThresholdMinutes int    `mapstructure:"stale_threshold_minutes"`
	AdapterServiceURL     string `mapstructure:"adapter_service_url"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string
}

// StaleThreshold returns StaleThresholdMinutes as a time.Duration.
func (c *Config) StaleThreshold() time.Duration {
	return time.Duration(c.API.StaleThresholdMinutes) * time.Minute
}

// Load reads config from the YAML file at path, then applies env-variable overrides.
// Env key mapping: viper key delimiter "." → "_", e.g. DATABASE_URL overrides database.url.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults so the service starts with sensible values even without a config file.
	v.SetDefault("log.level", "info")
	v.SetDefault("api.port", 8081)
	v.SetDefault("api.stale_threshold_minutes", 30)
	v.SetDefault("api.adapter_service_url", "http://adapter-service:8080")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("database.url is required")
	}
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		return nil, fmt.Errorf("api.port must be between 1 and 65535, got %d", cfg.API.Port)
	}
	if cfg.API.StaleThresholdMinutes < 0 {
		return nil, fmt.Errorf("api.stale_threshold_minutes must be >= 0, got %d", cfg.API.StaleThresholdMinutes)
	}

	return &cfg, nil
}
