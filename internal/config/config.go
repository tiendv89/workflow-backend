package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for api-service.
type Config struct {
	Port              int
	DatabaseURL       string
	AdapterServiceURL string
	StaleThreshold    time.Duration
}

// Load reads configuration from environment variables.
// Required: DATABASE_URL.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port := 8081
	if raw := os.Getenv("PORT"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		port = p
	}

	staleThreshold := 30 * time.Minute
	if raw := os.Getenv("STALE_THRESHOLD_MINUTES"); raw != "" {
		m, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid STALE_THRESHOLD_MINUTES: %w", err)
		}
		if m < 0 {
			return nil, fmt.Errorf("invalid STALE_THRESHOLD_MINUTES: must be >= 0")
		}
		staleThreshold = time.Duration(m) * time.Minute
	}

	adapterURL := os.Getenv("ADAPTER_SERVICE_URL")
	if adapterURL == "" {
		adapterURL = "http://adapter-service:8080"
	}

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		AdapterServiceURL: adapterURL,
		StaleThreshold:    staleThreshold,
	}, nil
}
