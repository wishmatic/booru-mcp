package config

import (
	"os"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	prev, had := os.LookupEnv(key)

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}

	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)

			return
		}

		_ = os.Unsetenv(key)
	})
}

func TestLoadDefaults(t *testing.T) {
	unsetEnv(t, "HOST")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", cfg.Host)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}

	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
}

func TestLoadValues(t *testing.T) {
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9000")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("API_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Host)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}

	if cfg.APIKey != "secret" {
		t.Errorf("APIKey = %q, want secret", cfg.APIKey)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-number")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
}

func TestAddr(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 9000}

	if got := cfg.Addr(); got != "127.0.0.1:9000" {
		t.Fatalf("Addr() = %q, want 127.0.0.1:9000", got)
	}
}
