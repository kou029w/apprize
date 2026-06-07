package config

import "testing"

func TestFromEnv_ConfigMaxLengthDefault(t *testing.T) {
	t.Setenv("APPRIZE_CONFIG_MAX_LENGTH", "")
	cfg := FromEnv()
	if cfg.ConfigMaxKB != 512 {
		t.Fatalf("ConfigMaxKB = %d, want 512", cfg.ConfigMaxKB)
	}
}

func TestFromEnv_ConfigMaxLengthFromEnv(t *testing.T) {
	t.Setenv("APPRIZE_CONFIG_MAX_LENGTH", "1024")
	cfg := FromEnv()
	if cfg.ConfigMaxKB != 1024 {
		t.Fatalf("ConfigMaxKB = %d, want 1024", cfg.ConfigMaxKB)
	}
}

func TestFromEnv_ConfigMaxLengthInvalidFallsBack(t *testing.T) {
	t.Setenv("APPRIZE_CONFIG_MAX_LENGTH", "0")
	cfg := FromEnv()
	if cfg.ConfigMaxKB != 512 {
		t.Fatalf("ConfigMaxKB = %d, want 512", cfg.ConfigMaxKB)
	}
}

func TestFromEnv_DefaultConfigIDDefault(t *testing.T) {
	t.Setenv("APPRIZE_DEFAULT_CONFIG_ID", "")
	cfg := FromEnv()
	if cfg.DefaultConfigID != "apprise" {
		t.Fatalf("DefaultConfigID = %q, want %q", cfg.DefaultConfigID, "apprise")
	}
}

func TestFromEnv_DefaultConfigIDFromEnv(t *testing.T) {
	t.Setenv("APPRIZE_DEFAULT_CONFIG_ID", "my-default")
	cfg := FromEnv()
	if cfg.DefaultConfigID != "my-default" {
		t.Fatalf("DefaultConfigID = %q, want %q", cfg.DefaultConfigID, "my-default")
	}
}
