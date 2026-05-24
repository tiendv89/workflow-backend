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
  http:
    address: ":8081"
    mode: release
  stale_threshold_minutes: 30
  adapter_service_url: "http://adapter-service:8080"
db:
  host: localhost
  port: 5432
  db_name: testdb
  user: testuser
  password: testpass
`

// TestConfig_RequiresDBHost verifies that missing db.host is rejected at startup.
func TestConfig_RequiresDBHost(t *testing.T) {
	p := writeConfig(t, `
log:
  level: info
api:
  http:
    address: ":8081"
    mode: release
db:
  host: ""
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error when db.host is empty")
	}
}

// TestConfig_Defaults verifies that defaults are applied when keys are absent.
func TestConfig_Defaults(t *testing.T) {
	p := writeConfig(t, baseConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.API.HTTP.Address != ":8081" {
		t.Errorf("expected api.http.address=:8081, got %q", cfg.API.HTTP.Address)
	}
	if cfg.API.StaleThresholdMinutes != 30 {
		t.Errorf("expected default stale threshold 30, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://adapter-service:8080" {
		t.Errorf("expected default adapter URL, got %q", cfg.API.AdapterServiceURL)
	}
}

// TestConfig_CustomAddress verifies api.http.address is parsed correctly from YAML.
func TestConfig_CustomAddress(t *testing.T) {
	p := writeConfig(t, baseConfig+`
api:
  http:
    address: ":9090"
    mode: debug
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.API.HTTP.Address != ":9090" {
		t.Errorf("expected api.http.address=:9090, got %q", cfg.API.HTTP.Address)
	}
	if cfg.API.HTTP.Mode != "debug" {
		t.Errorf("expected api.http.mode=debug, got %q", cfg.API.HTTP.Mode)
	}
}

// TestConfig_EnvOverride verifies that env vars override YAML values.
func TestConfig_EnvOverride(t *testing.T) {
	t.Setenv("DB_HOST", "envhost")
	p := writeConfig(t, baseConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DB.Host != "envhost" {
		t.Errorf("expected env override for db.host, got %q", cfg.DB.Host)
	}
}

// TestConfig_NegativeStaleThreshold verifies negative stale threshold is rejected.
func TestConfig_NegativeStaleThreshold(t *testing.T) {
	p := writeConfig(t, baseConfig+`
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
	p := writeConfig(t, baseConfig+`
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
