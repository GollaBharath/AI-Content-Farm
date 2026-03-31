package config

import (
	"os"
	"testing"
)

func TestEnvIntOrDefault(t *testing.T) {
	os.Setenv("TEST_INT_VAL", "42")
	defer os.Unsetenv("TEST_INT_VAL")

	if val := envIntOrDefault("TEST_INT_VAL", 10); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	if val := envIntOrDefault("TEST_INT_MISSING", 10); val != 10 {
		t.Errorf("expected 10, got %d", val)
	}

	os.Setenv("TEST_INT_INVALID", "invalid")
	defer os.Unsetenv("TEST_INT_INVALID")
	if val := envIntOrDefault("TEST_INT_INVALID", 10); val != 10 {
		t.Errorf("expected fallback 10 for invalid int, got %d", val)
	}
}

func TestEnvOrDefault(t *testing.T) {
	os.Setenv("TEST_STR_VAL", "hello")
	defer os.Unsetenv("TEST_STR_VAL")

	if val := envOrDefault("TEST_STR_VAL", "world"); val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}

	if val := envOrDefault("TEST_STR_MISSING", "world"); val != "world" {
		t.Errorf("expected 'world', got %q", val)
	}
}

func TestEnvBoolOrDefault(t *testing.T) {
	os.Setenv("TEST_BOOL_VAL", "true")
	defer os.Unsetenv("TEST_BOOL_VAL")

	if val := envBoolOrDefault("TEST_BOOL_VAL", false); val != true {
		t.Errorf("expected true, got %v", val)
	}

	if val := envBoolOrDefault("TEST_BOOL_MISSING", false); val != false {
		t.Errorf("expected false, got %v", val)
	}

	os.Setenv("TEST_BOOL_INVALID", "invalid")
	defer os.Unsetenv("TEST_BOOL_INVALID")
	if val := envBoolOrDefault("TEST_BOOL_INVALID", true); val != true {
		t.Errorf("expected fallback true for invalid bool, got %v", val)
	}
}

func TestLoadDefaults(t *testing.T) {
	// Unset variables that might interfere with defaults testing
	os.Unsetenv("PORT")
	os.Unsetenv("TTS_PROVIDER")
	
	// Set required ones
	os.Setenv("INPUT_VIDEOS_DIR", "/tmp/in")
	os.Setenv("OUTPUT_VIDEOS_DIR", "/tmp/out")
	defer os.Unsetenv("INPUT_VIDEOS_DIR")
	defer os.Unsetenv("OUTPUT_VIDEOS_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got %q", cfg.Port)
	}
	if cfg.TTSProvider != "piper" {
		t.Errorf("expected default TTSProvider 'piper', got %q", cfg.TTSProvider)
	}
}
