package integration_test

import (
	"os"
	"testing"

	"github.com/tiendv89/workflow-backend/internal/config"
)

// TestConfig_RequiresDatabaseURL verifies that missing DATABASE_URL is rejected at startup.
func TestConfig_RequiresDatabaseURL(t *testing.T) {
	_ = os.Unsetenv("DATABASE_URL")
	_, err := config.Load()
	if err == nil {
		t.Error("expected error when DATABASE_URL is missing")
	}
}

// TestConfig_Defaults verifies that optional env vars have correct defaults.
func TestConfig_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("STALE_THRESHOLD_MINUTES")
	_ = os.Unsetenv("ADAPTER_SERVICE_URL")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8081 {
		t.Errorf("expected default port 8081, got %d", cfg.Port)
	}
	if cfg.StaleThreshold.Minutes() != 30 {
		t.Errorf("expected default stale threshold 30m, got %v", cfg.StaleThreshold)
	}
	if cfg.AdapterServiceURL != "http://adapter-service:8080" {
		t.Errorf("expected default adapter URL, got %q", cfg.AdapterServiceURL)
	}
}

// TestConfig_CustomPort verifies the PORT env var is parsed correctly.
func TestConfig_CustomPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
}

// TestConfig_InvalidPort returns an error for non-numeric PORT.
func TestConfig_InvalidPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("PORT", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid PORT value")
	}
}

// TestConfig_CustomStaleThreshold verifies STALE_THRESHOLD_MINUTES is parsed.
func TestConfig_CustomStaleThreshold(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("STALE_THRESHOLD_MINUTES", "60")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StaleThreshold.Minutes() != 60 {
		t.Errorf("expected stale threshold 60m, got %v", cfg.StaleThreshold)
	}
}

// TestConfig_InvalidStaleThreshold rejects negative values.
func TestConfig_InvalidStaleThreshold(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/test")
	t.Setenv("STALE_THRESHOLD_MINUTES", "-1")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for negative STALE_THRESHOLD_MINUTES value")
	}
}
