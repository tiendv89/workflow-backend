package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiendv89/workflow-backend/configs"
)

// writeConfig writes a temporary config.yaml for tests and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	_ = f.Close()
	return filepath.Clean(f.Name())
}

const baseConfig = `
log:
  level: info
api:
  port: 8081
  stale_threshold_minutes: 30
  adapter_service_url: "http://adapter-service:8080"
database:
  url: "postgresql://localhost/test"
`

// TestConfig_RequiresDatabaseURL verifies that missing database.url is rejected at startup.
func TestConfig_RequiresDatabaseURL(t *testing.T) {
	p := writeConfig(t, `
log:
  level: info
api:
  port: 8081
database:
  url: ""
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error when database.url is empty")
	}
}

// TestConfig_Defaults verifies that defaults are applied when keys are absent.
func TestConfig_Defaults(t *testing.T) {
	p := writeConfig(t, `
database:
  url: "postgresql://localhost/test"
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.API.Port != 8081 {
		t.Errorf("expected default port 8081, got %d", cfg.API.Port)
	}
	if cfg.API.StaleThresholdMinutes != 30 {
		t.Errorf("expected default stale threshold 30, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://adapter-service:8080" {
		t.Errorf("expected default adapter URL, got %q", cfg.API.AdapterServiceURL)
	}
}

// TestConfig_CustomPort verifies the api.port key is parsed correctly from YAML.
func TestConfig_CustomPort(t *testing.T) {
	p := writeConfig(t, `
database:
  url: "postgresql://localhost/test"
api:
  port: 9090
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.API.Port)
	}
}

// TestConfig_EnvOverride verifies that env vars override YAML values.
func TestConfig_EnvOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://envhost/envdb")
	p := writeConfig(t, baseConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.URL != "postgresql://envhost/envdb" {
		t.Errorf("expected env override for database.url, got %q", cfg.Database.URL)
	}
}

// TestConfig_InvalidPort verifies port validation.
func TestConfig_InvalidPort(t *testing.T) {
	p := writeConfig(t, `
database:
  url: "postgresql://localhost/test"
api:
  port: 99999
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error for out-of-range port")
	}
}

// TestConfig_NegativeStaleThreshold verifies negative stale threshold is rejected.
func TestConfig_NegativeStaleThreshold(t *testing.T) {
	p := writeConfig(t, `
database:
  url: "postgresql://localhost/test"
api:
  stale_threshold_minutes: -1
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error for negative stale_threshold_minutes")
	}
}

// TestConfig_StaleThresholdDuration verifies StaleThreshold() returns the right duration.
func TestConfig_StaleThresholdDuration(t *testing.T) {
	p := writeConfig(t, `
database:
  url: "postgresql://localhost/test"
api:
  stale_threshold_minutes: 60
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StaleThreshold().Minutes() != 60 {
		t.Errorf("expected 60 minutes, got %v", cfg.StaleThreshold())
	}
}
