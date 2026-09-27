package config

import (
	"testing"
	"time"
)

func TestValidateDefaultsSecretsInDevelopment(t *testing.T) {
	cfg := &Config{AppEnv: "development", OrgTimezone: "UTC"}
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTAccessSecret == "" || cfg.JWTRefreshSecret == "" {
		t.Fatal("expected development secrets to be populated")
	}
}

func TestValidateRequiresSecretsInProduction(t *testing.T) {
	cfg := &Config{AppEnv: "production", OrgTimezone: "UTC"}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when production secrets are missing")
	}
}

func TestValidateRejectsUnknownTimezone(t *testing.T) {
	cfg := &Config{AppEnv: "development", OrgTimezone: "Not/AZone"}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestValidateDefaultsCORSOriginsInDevelopment(t *testing.T) {
	cfg := &Config{AppEnv: "development", OrgTimezone: "UTC"}
	if err := cfg.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("expected localhost default, got %v", cfg.CORSAllowedOrigins)
	}
}

func TestValidateRequiresCORSOriginsInProduction(t *testing.T) {
	cfg := &Config{
		AppEnv:           "production",
		OrgTimezone:      "UTC",
		JWTAccessSecret:  "access",
		JWTRefreshSecret: "refresh",
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when CORS origins are missing in production")
	}
}

func TestSplitList(t *testing.T) {
	got := splitList("http://a.example.com, http://b.example.com ,,")
	want := []string{"http://a.example.com", "http://b.example.com"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestDurationEnv(t *testing.T) {
	t.Setenv("TEST_TTL", "90m")
	got, err := durationEnv("TEST_TTL", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 90*time.Minute {
		t.Fatalf("expected 90m, got %s", got)
	}

	t.Setenv("TEST_TTL", "30")
	got, err = durationEnv("TEST_TTL", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 30*time.Second {
		t.Fatalf("expected 30s, got %s", got)
	}
}
