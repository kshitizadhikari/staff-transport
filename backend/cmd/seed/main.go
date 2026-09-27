// Command seed provisions local development accounts.
//
// It is idempotent: running it again updates the existing account's password,
// name, role, and status. Configure it with SEED_MANAGER_* environment
// variables. Never use the default password outside local development.
package main

import (
	"context"
	"log/slog"
	"os"

	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/users"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	gdb, err := db.Open(ctx, cfg)
	if err != nil {
		return err
	}

	name := env("SEED_MANAGER_NAME", "Demo Manager")
	email := env("SEED_MANAGER_EMAIL", "manager@example.com")
	password := env("SEED_MANAGER_PASSWORD", "changeme123")

	svc := users.NewService(users.NewRepository(gdb))
	user, err := svc.Provision(ctx, name, email, password, users.RoleManager)
	if err != nil {
		return err
	}

	slog.Info("seeded manager account", "id", user.ID, "email", email, "role", user.Role)
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
