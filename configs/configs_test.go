package configs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiendv89/workflow-backend/configs"
)

func writeConfigFile(t *testing.T, content string) string {
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

func TestLoad_MissingFile(t *testing.T) {
	_, err := configs.Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	p := writeConfigFile(t, `
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

func TestLoad_ValidConfig(t *testing.T) {
	p := writeConfigFile(t, `
log:
  level: debug
api:
  port: 9090
  stale_threshold_minutes: 60
  adapter_service_url: "http://my-adapter:8000"
database:
  url: "postgresql://localhost/testdb"
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log.level=debug, got %q", cfg.Log.Level)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("expected api.port=9090, got %d", cfg.API.Port)
	}
	if cfg.API.StaleThresholdMinutes != 60 {
		t.Errorf("expected stale_threshold_minutes=60, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://my-adapter:8000" {
		t.Errorf("expected adapter_service_url, got %q", cfg.API.AdapterServiceURL)
	}
	if cfg.Database.URL != "postgresql://localhost/testdb" {
		t.Errorf("expected database.url, got %q", cfg.Database.URL)
	}
}

func TestLoad_Defaults(t *testing.T) {
	p := writeConfigFile(t, `
database:
  url: "postgresql://localhost/test"
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected default log.level=info, got %q", cfg.Log.Level)
	}
	if cfg.API.Port != 8081 {
		t.Errorf("expected default port=8081, got %d", cfg.API.Port)
	}
	if cfg.API.StaleThresholdMinutes != 30 {
		t.Errorf("expected default stale_threshold_minutes=30, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://adapter-service:8080" {
		t.Errorf("expected default adapter_service_url, got %q", cfg.API.AdapterServiceURL)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	p := writeConfigFile(t, `
database:
  url: "postgresql://localhost/test"
api:
  port: 99999
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error for port > 65535")
	}
}

func TestLoad_NegativeStaleThreshold(t *testing.T) {
	p := writeConfigFile(t, `
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

func TestLoad_StaleThresholdDuration(t *testing.T) {
	p := writeConfigFile(t, `
database:
  url: "postgresql://localhost/test"
api:
  stale_threshold_minutes: 45
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 45 * time.Minute
	if cfg.StaleThreshold() != expected {
		t.Errorf("expected %v, got %v", expected, cfg.StaleThreshold())
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://envhost/envdb")
	p := writeConfigFile(t, `
database:
  url: "postgresql://yaml-host/yaml-db"
`)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.URL != "postgresql://envhost/envdb" {
		t.Errorf("expected env override, got %q", cfg.Database.URL)
	}
}
