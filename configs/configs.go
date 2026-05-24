// Package configs provides viper-backed YAML config loading with env-variable overrides.
package configs

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/tiendv89/workflow-backend/pkg/db"
)

// G is the global config instance populated by Init.
var G *Config

// Config holds all runtime configuration for api-service.
type Config struct {
	Log LogConfig
	API APIConfig
	DB  db.Config `mapstructure:"db"`
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

// Init loads config from path into the global G variable, panicking on error.
func Init(path string) {
	cfg, err := Load(path)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	G = cfg
}

// StaleThreshold returns StaleThresholdMinutes as a time.Duration.
func (c *Config) StaleThreshold() time.Duration {
	return time.Duration(c.API.StaleThresholdMinutes) * time.Minute
}

// Load reads config from the YAML file at path, then applies env-variable overrides.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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

	if cfg.DB.Host == "" {
		return nil, fmt.Errorf("db.host is required")
	}
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		return nil, fmt.Errorf("api.port must be between 1 and 65535, got %d", cfg.API.Port)
	}
	if cfg.API.StaleThresholdMinutes < 0 {
		return nil, fmt.Errorf("api.stale_threshold_minutes must be >= 0, got %d", cfg.API.StaleThresholdMinutes)
	}

	return &cfg, nil
}
