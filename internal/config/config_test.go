package config

import (
	"os"
	"testing"
	"time"
)

func TestDefault_HasExpectedValues(t *testing.T) {
	cfg := Default()
	if cfg.Cache.ReadTTL != "24h" {
		t.Fatalf("expected ReadTTL 24h, got %s", cfg.Cache.ReadTTL)
	}
	if cfg.Output.DefaultFormat != "json" {
		t.Fatalf("expected DefaultFormat json, got %s", cfg.Output.DefaultFormat)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Fatalf("expected MaxAttempts 3, got %d", cfg.Retry.MaxAttempts)
	}
}

func TestReadTTL(t *testing.T) {
	cfg := Default()
	if cfg.ReadTTL() != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", cfg.ReadTTL())
	}
}

func TestSearchTTL(t *testing.T) {
	cfg := Default()
	if cfg.SearchTTL() != time.Hour {
		t.Fatalf("expected 1h, got %v", cfg.SearchTTL())
	}
}

func TestBackendTimeout_Default(t *testing.T) {
	cfg := Default()
	d := cfg.BackendTimeout("direct")
	if d != 20*time.Second {
		t.Fatalf("expected 20s, got %v", d)
	}
}

func TestBackendTimeout_Unknown(t *testing.T) {
	cfg := Default()
	d := cfg.BackendTimeout("unknown-backend")
	if d != 30*time.Second {
		t.Fatalf("expected 30s fallback, got %v", d)
	}
}

func TestApplyEnv_DefaultFormat(t *testing.T) {
	os.Setenv("WEBSCOUT_DEFAULT_FORMAT", "markdown")
	defer os.Unsetenv("WEBSCOUT_DEFAULT_FORMAT")
	cfg := Default()
	applyEnv(cfg)
	if cfg.Output.DefaultFormat != "markdown" {
		t.Fatalf("expected markdown, got %s", cfg.Output.DefaultFormat)
	}
}

func TestNormalize_SetsDefaultFormat(t *testing.T) {
	cfg := Default()
	cfg.Output.DefaultFormat = ""
	if err := normalize(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Output.DefaultFormat != "json" {
		t.Fatalf("expected json after normalize, got %s", cfg.Output.DefaultFormat)
	}
}
