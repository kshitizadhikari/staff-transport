package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv  string
	Port    string
	BaseURL string

	DatabaseURL string
	RedisURL    string

	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	OrgTimezone string

	CORSAllowedOrigins []string

	GoogleMapsAPIKey  string
	MapboxAccessToken string
	MapboxCountry     string
	ExpoAccessToken   string
}

func Load() (*Config, error) {
	loadDotEnv()

	accessTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := durationEnv("REFRESH_TOKEN_TTL", 168*time.Hour)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:  env("APP_ENV", "development"),
		Port:    env("APP_PORT", "8080"),
		BaseURL: env("APP_BASE_URL", "http://localhost:8080"),

		DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5434/staff_transport?sslmode=disable"),
		RedisURL:    env("REDIS_URL", "redis://localhost:6379/0"),

		JWTAccessSecret:  env("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret: env("JWT_REFRESH_SECRET", ""),
		AccessTokenTTL:   accessTTL,
		RefreshTokenTTL:  refreshTTL,

		OrgTimezone: env("ORG_TIMEZONE", "UTC"),

		CORSAllowedOrigins: splitList(env("CORS_ALLOWED_ORIGINS", "")),

		GoogleMapsAPIKey:  env("GOOGLE_MAPS_API_KEY", ""),
		MapboxAccessToken: env("MAPBOX_ACCESS_TOKEN", ""),
		MapboxCountry:     env("MAPBOX_COUNTRY", "np"),
		ExpoAccessToken:   env("EXPO_ACCESS_TOKEN", ""),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func (c *Config) validate() error {
	if c.IsProduction() {
		if c.JWTAccessSecret == "" || c.JWTRefreshSecret == "" {
			return fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET are required in production")
		}
	}
	if c.JWTAccessSecret == "" {
		c.JWTAccessSecret = "development-access-secret"
	}
	if c.JWTRefreshSecret == "" {
		c.JWTRefreshSecret = "development-refresh-secret"
	}
	if len(c.CORSAllowedOrigins) == 0 {
		if c.IsProduction() {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS is required in production")
		}
		c.CORSAllowedOrigins = []string{"http://localhost:3000"}
	}
	if _, err := time.LoadLocation(c.OrgTimezone); err != nil {
		return fmt.Errorf("invalid ORG_TIMEZONE %q: %w", c.OrgTimezone, err)
	}
	return nil
}

func loadDotEnv() {
	for _, path := range []string{".env", "../.env"} {
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			return
		}
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %q", key, raw)
	}
	return time.Duration(seconds) * time.Second, nil
}
