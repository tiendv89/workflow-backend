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

const validDBConfig = `
db:
  host: 127.0.0.1
  port: 5432
  db_name: testdb
  user: testuser
  password: testpass
  conn_life_time_seconds: 300
  max_idle_conns: 10
  max_open_conns: 30
  log_level: 1
  auto_migration: false
  migration_dir: "file://migrations"
`

func TestLoad_MissingDBHost(t *testing.T) {
	p := writeConfigFile(t, `
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

func TestLoad_ValidConfig(t *testing.T) {
	p := writeConfigFile(t, `
log:
  level: debug
api:
  http:
    address: ":9090"
    mode: debug
  stale_threshold_minutes: 60
  adapter_service_url: "http://my-adapter:8000"`+validDBConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log.level=debug, got %q", cfg.Log.Level)
	}
	if cfg.API.HTTP.Address != ":9090" {
		t.Errorf("expected api.http.address=:9090, got %q", cfg.API.HTTP.Address)
	}
	if cfg.API.HTTP.Mode != "debug" {
		t.Errorf("expected api.http.mode=debug, got %q", cfg.API.HTTP.Mode)
	}
	if cfg.API.StaleThresholdMinutes != 60 {
		t.Errorf("expected stale_threshold_minutes=60, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://my-adapter:8000" {
		t.Errorf("expected adapter_service_url, got %q", cfg.API.AdapterServiceURL)
	}
	if cfg.DB.Host != "127.0.0.1" {
		t.Errorf("expected db.host=127.0.0.1, got %q", cfg.DB.Host)
	}
	expectedDSN := "postgres://testuser:testpass@127.0.0.1:5432/testdb?sslmode=disable"
	if cfg.DB.DSN() != expectedDSN {
		t.Errorf("expected DSN %q, got %q", expectedDSN, cfg.DB.DSN())
	}
}

func TestLoad_Defaults(t *testing.T) {
	p := writeConfigFile(t, validDBConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected default log.level=info, got %q", cfg.Log.Level)
	}
	if cfg.API.HTTP.Address != ":8081" {
		t.Errorf("expected default api.http.address=:8081, got %q", cfg.API.HTTP.Address)
	}
	if cfg.API.HTTP.Mode != "release" {
		t.Errorf("expected default api.http.mode=release, got %q", cfg.API.HTTP.Mode)
	}
	if cfg.API.StaleThresholdMinutes != 30 {
		t.Errorf("expected default stale_threshold_minutes=30, got %d", cfg.API.StaleThresholdMinutes)
	}
	if cfg.API.AdapterServiceURL != "http://adapter-service:8080" {
		t.Errorf("expected default adapter_service_url, got %q", cfg.API.AdapterServiceURL)
	}
}

func TestLoad_NegativeStaleThreshold(t *testing.T) {
	p := writeConfigFile(t, validDBConfig+`
api:
  stale_threshold_minutes: -1
`)
	_, err := configs.Load(p)
	if err == nil {
		t.Error("expected error for negative stale_threshold_minutes")
	}
}

func TestLoad_StaleThresholdDuration(t *testing.T) {
	p := writeConfigFile(t, validDBConfig+`
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
	t.Setenv("DB_HOST", "envhost")
	p := writeConfigFile(t, validDBConfig)
	cfg, err := configs.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DB.Host != "envhost" {
		t.Errorf("expected env override db.host=envhost, got %q", cfg.DB.Host)
	}
}
