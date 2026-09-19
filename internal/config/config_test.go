package config

import "testing"

func TestLoadDefault(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("PORT", "")
	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("addr = %q, want %q", cfg.Addr, ":8080")
	}
}

func TestLoadADDR(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("PORT", "")
	cfg := Load()
	if cfg.Addr != ":9090" {
		t.Fatalf("addr = %q, want %q", cfg.Addr, ":9090")
	}
}

func TestLoadPORT(t *testing.T) {
	t.Setenv("ADDR", ":8080")
	t.Setenv("PORT", "3000")
	cfg := Load()
	if cfg.Addr != ":3000" {
		t.Fatalf("addr = %q, want %q", cfg.Addr, ":3000")
	}
}
