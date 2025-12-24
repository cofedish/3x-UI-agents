package config

import "testing"

func TestLoadConfigUsesJWTTokenFallback(t *testing.T) {
	t.Setenv("AGENT_AUTH_TYPE", "jwt")
	t.Setenv("AGENT_JWT_TOKEN", "test-token")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.JWTSecret != "test-token" {
		t.Fatalf("expected JWTSecret to use AGENT_JWT_TOKEN, got %q", cfg.JWTSecret)
	}
}
