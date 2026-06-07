package config

import "testing"

func TestFromEnv_BindDefault(t *testing.T) {
	t.Setenv("APPRIZE_BIND", "")
	t.Setenv("HTTP_PORT", "")
	cfg := FromEnv()
	if cfg.Bind != ":8000" {
		t.Fatalf("Bind = %q, want %q", cfg.Bind, ":8000")
	}
}

func TestFromEnv_BindFromHTTPPort(t *testing.T) {
	t.Setenv("APPRIZE_BIND", "")
	t.Setenv("HTTP_PORT", "9000")
	cfg := FromEnv()
	if cfg.Bind != ":9000" {
		t.Fatalf("Bind = %q, want %q", cfg.Bind, ":9000")
	}
}

func TestFromEnv_RecursionMaxDefault(t *testing.T) {
	t.Setenv("APPRIZE_RECURSION_MAX", "")
	cfg := FromEnv()
	if cfg.RecursionMax != 1 {
		t.Fatalf("RecursionMax = %d, want 1", cfg.RecursionMax)
	}
}
