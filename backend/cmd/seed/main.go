// Command seed provisions local development data: a manager account, ten
// drivers, and ten vehicles.
//
// It is idempotent: the manager account is upserted, and drivers/vehicles that
// already exist (by email/registration) are skipped. Configure it with the
// SEED_* environment variables. Never use the default passwords outside local
// development.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"gorm.io/gorm"

	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/drivers"
	"staff-transport/internal/users"
	"staff-transport/internal/vehicles"
)

const seedCount = 10

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

	if err := seedManager(ctx, gdb); err != nil {
		return err
	}
	driverCreated, driverSkipped, err := seedDrivers(ctx, gdb)
	if err != nil {
		return err
	}
	vehicleCreated, vehicleSkipped, err := seedVehicles(ctx, gdb)
	if err != nil {
		return err
	}

	slog.Info("seed complete",
		"drivers_created", driverCreated, "drivers_skipped", driverSkipped,
		"vehicles_created", vehicleCreated, "vehicles_skipped", vehicleSkipped,
	)
	return nil
}

func seedManager(ctx context.Context, gdb *gorm.DB) error {
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

func seedDrivers(ctx context.Context, gdb *gorm.DB) (created, skipped int, err error) {
	password := env("SEED_DRIVER_PASSWORD", "driverpass123")
	svc := drivers.NewService(drivers.NewRepository(gdb))

	for i := 1; i <= seedCount; i++ {
		email := fmt.Sprintf("driver%d@example.com", i)
		name := fmt.Sprintf("Driver %d", i)
		_, createErr := svc.Create(ctx, drivers.CreateInput{
			Name:   name,
			Email:  &email,
			Status: drivers.StatusAvailable,
		}, password)
		switch {
		case createErr == nil:
			created++
			slog.Info("seeded driver", "email", email)
		case errors.Is(createErr, drivers.ErrEmailTaken), db.IsUniqueViolation(createErr):
			skipped++
			slog.Info("driver already exists, skipping", "email", email)
		default:
			return created, skipped, createErr
		}
	}
	return created, skipped, nil
}

func seedVehicles(ctx context.Context, gdb *gorm.DB) (created, skipped int, err error) {
	svc := vehicles.NewService(vehicles.NewRepository(gdb))

	for i := 1; i <= seedCount; i++ {
		registration := fmt.Sprintf("TEST-VEH-%02d", i)
		model := "Toyota HiAce"
		_, createErr := svc.Create(ctx, vehicles.CreateInput{
			RegistrationNumber: registration,
			Model:              &model,
			Capacity:           14,
			Status:             vehicles.StatusAvailable,
		})
		switch {
		case createErr == nil:
			created++
			slog.Info("seeded vehicle", "registration", registration)
		case errors.Is(createErr, vehicles.ErrRegistrationTaken), db.IsUniqueViolation(createErr):
			skipped++
			slog.Info("vehicle already exists, skipping", "registration", registration)
		default:
			return created, skipped, createErr
		}
	}
	return created, skipped, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
