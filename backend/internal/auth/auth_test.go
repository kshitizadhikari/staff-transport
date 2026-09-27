package auth

import (
	"errors"
	"testing"
	"time"

	"staff-transport/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		AppEnv:          "development",
		JWTAccessSecret: "test-access-secret",
		AccessTokenTTL:  time.Minute,
	}
}

func TestParseAccessTokenRoundTrip(t *testing.T) {
	svc := NewService(testConfig(), nil).(*service)

	token, err := svc.signAccessToken("user-1", "manager")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := svc.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "manager" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseAccessTokenRejectsTamperedToken(t *testing.T) {
	svc := NewService(testConfig(), nil).(*service)

	token, err := svc.signAccessToken("user-1", "staff")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := svc.ParseAccessToken(token + "x"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseAccessTokenRejectsExpiredToken(t *testing.T) {
	cfg := testConfig()
	cfg.AccessTokenTTL = -time.Minute
	svc := NewService(cfg, nil).(*service)

	token, err := svc.signAccessToken("user-1", "staff")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := svc.ParseAccessToken(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}
