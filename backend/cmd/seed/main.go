// Command seed provisions local development data: a manager account, ten staff,
// ten drivers, and ten vehicles, all with realistic profile details.
//
// It is idempotent: the manager account is upserted, and staff/drivers/vehicles
// that already exist (by email/registration) are skipped. Configure it with the
// SEED_* environment variables. Never use the default passwords outside local
// development.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"gorm.io/gorm"

	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/drivers"
	"staff-transport/internal/maps"
	"staff-transport/internal/staff"
	"staff-transport/internal/users"
	"staff-transport/internal/vehicles"
)

type staffSeed struct {
	Name        string
	Email       string
	Phone       string
	Department  string
	HomeAddress string
}

type driverSeed struct {
	Name          string
	Email         string
	Phone         string
	LicenseNumber string
	LicenseExpiry string
}

type vehicleSeed struct {
	Registration string
	Model        string
	Capacity     int
}

var staffSeeds = []staffSeed{
	{"Aarav Sharma", "aarav.sharma@stafftrans.example.com", "+977-9801234501", "Engineering", "Baluwatar, Kathmandu"},
	{"Priya Adhikari", "priya.adhikari@stafftrans.example.com", "+977-9801234502", "Finance", "Patan, Lalitpur"},
	{"Rohan Thapa", "rohan.thapa@stafftrans.example.com", "+977-9801234503", "Operations", "Thamel, Kathmandu"},
	{"Sneha Gurung", "sneha.gurung@stafftrans.example.com", "+977-9801234504", "Human Resources", "Lakeside, Pokhara"},
	{"Kiran Shrestha", "kiran.shrestha@stafftrans.example.com", "+977-9801234505", "Engineering", "New Baneshwor, Kathmandu"},
	{"Anisha Rai", "anisha.rai@stafftrans.example.com", "+977-9801234506", "Marketing", "Boudha, Kathmandu"},
	{"Bikash Magar", "bikash.magar@stafftrans.example.com", "+977-9801234507", "Sales", "Suryabinayak, Bhaktapur"},
	{"Nisha Tamang", "nisha.tamang@stafftrans.example.com", "+977-9801234508", "Finance", "Jhamsikhel, Lalitpur"},
	{"Sujan Karki", "sujan.karki@stafftrans.example.com", "+977-9801234509", "Operations", "Koteshwor, Kathmandu"},
	{"Pooja Bhattarai", "pooja.bhattarai@stafftrans.example.com", "+977-9801234510", "Engineering", "Dillibazar, Kathmandu"},
}

var driverSeeds = []driverSeed{
	{"Rajesh Lama", "rajesh.lama@stafftrans.example.com", "+977-9812345601", "LAL-2018-1001", "2027-04-12"},
	{"Suresh Yadav", "suresh.yadav@stafftrans.example.com", "+977-9812345602", "LAL-2017-2043", "2026-11-30"},
	{"Dipak Bhandari", "dipak.bhandari@stafftrans.example.com", "+977-9812345603", "LAL-2019-3387", "2028-02-15"},
	{"Manoj Chaudhary", "manoj.chaudhary@stafftrans.example.com", "+977-9812345604", "LAL-2016-4521", "2026-09-05"},
	{"Prakash Limbu", "prakash.limbu@stafftrans.example.com", "+977-9812345605", "LAL-2020-5198", "2029-06-20"},
	{"Gopal Neupane", "gopal.neupane@stafftrans.example.com", "+977-9812345606", "LAL-2015-6032", "2026-12-01"},
	{"Hari Bahadur Shrestha", "hari.shrestha@stafftrans.example.com", "+977-9812345607", "LAL-2021-7114", "2030-03-18"},
	{"Santosh Poudel", "santosh.poudel@stafftrans.example.com", "+977-9812345608", "LAL-2018-8250", "2027-08-09"},
	{"Anil Maharjan", "anil.maharjan@stafftrans.example.com", "+977-9812345609", "LAL-2019-9366", "2028-05-27"},
	{"Ramesh Basnet", "ramesh.basnet@stafftrans.example.com", "+977-9812345610", "LAL-2014-1482", "2026-10-14"},
}

var vehicleSeeds = []vehicleSeed{
	{"BA 1 PA 1234", "Toyota HiAce", 14},
	{"BA 2 CHA 5678", "Ford Transit", 15},
	{"BA 3 KHA 9012", "Mahindra Scorpio", 7},
	{"LU 1 PA 3456", "Toyota Land Cruiser", 7},
	{"LU 2 JA 7890", "Suzuki Ertiga", 6},
	{"GA 1 KHA 2345", "Tata Winger", 13},
	{"BA 4 CHA 6789", "Hyundai H1", 9},
	{"BA 5 PA 1122", "Force Traveller", 17},
	{"KA 1 KHA 3344", "Toyota HiAce Commuter", 13},
	{"CO 1 PA 5566", "Toyota Coaster", 26},
}

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

	geocoder := maps.NewMapboxGeocoder(cfg.MapboxAccessToken, cfg.MapboxCountry)

	if err := seedManager(ctx, gdb); err != nil {
		return err
	}

	staffCreated, staffSkipped, err := seedStaff(ctx, gdb, geocoder)
	if err != nil {
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
		"staff_created", staffCreated, "staff_skipped", staffSkipped,
		"drivers_created", driverCreated, "drivers_skipped", driverSkipped,
		"vehicles_created", vehicleCreated, "vehicles_skipped", vehicleSkipped,
	)
	return nil
}

func seedManager(ctx context.Context, gdb *gorm.DB) error {
	name := env("SEED_MANAGER_NAME", "Anish Rana")
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

func seedStaff(ctx context.Context, gdb *gorm.DB, geocoder *maps.MapboxGeocoder) (created, skipped int, err error) {
	password := env("SEED_STAFF_PASSWORD", "staffpass123")
	svc := staff.NewService(staff.NewRepository(gdb), geocoder)

	for _, seed := range staffSeeds {
		email, phone, department, address := seed.Email, seed.Phone, seed.Department, seed.HomeAddress
		_, createErr := svc.Create(ctx, staff.CreateInput{
			Name:        seed.Name,
			Email:       &email,
			Phone:       &phone,
			Department:  &department,
			HomeAddress: &address,
			Active:      true,
		}, password)
		switch {
		case createErr == nil:
			created++
			slog.Info("seeded staff", "email", seed.Email, "department", seed.Department)
		case errors.Is(createErr, staff.ErrEmailTaken), db.IsUniqueViolation(createErr):
			// Refresh existing staff so addresses are (re)geocoded.
			id, lookupErr := staffIDByEmail(ctx, gdb, seed.Email)
			if lookupErr != nil {
				return created, skipped, lookupErr
			}
			name := seed.Name
			_, updateErr := svc.Update(ctx, id, staff.UpdateInput{
				Name:        &name,
				Phone:       &phone,
				Department:  &department,
				HomeAddress: &address,
			})
			if updateErr != nil {
				return created, skipped, updateErr
			}
			skipped++
			slog.Info("staff already exists, refreshed", "email", seed.Email)
		default:
			return created, skipped, createErr
		}
	}
	return created, skipped, nil
}

func staffIDByEmail(ctx context.Context, gdb *gorm.DB, email string) (string, error) {
	var id string
	err := gdb.WithContext(ctx).
		Table("staff").
		Select("staff.id").
		Joins("JOIN users u ON u.id = staff.user_id").
		Where("u.email = ?", email).
		Scan(&id).Error
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("staff %s not found after conflict", email)
	}
	return id, nil
}

func seedDrivers(ctx context.Context, gdb *gorm.DB) (created, skipped int, err error) {
	password := env("SEED_DRIVER_PASSWORD", "driverpass123")
	svc := drivers.NewService(drivers.NewRepository(gdb))

	for _, seed := range driverSeeds {
		expiry, parseErr := time.Parse("2006-01-02", seed.LicenseExpiry)
		if parseErr != nil {
			return created, skipped, fmt.Errorf("invalid license expiry for %s: %w", seed.Email, parseErr)
		}
		email, phone, license := seed.Email, seed.Phone, seed.LicenseNumber
		_, createErr := svc.Create(ctx, drivers.CreateInput{
			Name:          seed.Name,
			Email:         &email,
			Phone:         &phone,
			LicenseNumber: &license,
			LicenseExpiry: &expiry,
			Status:        drivers.StatusAvailable,
		}, password)
		switch {
		case createErr == nil:
			created++
			slog.Info("seeded driver", "email", seed.Email, "license", seed.LicenseNumber)
		case errors.Is(createErr, drivers.ErrEmailTaken), db.IsUniqueViolation(createErr):
			skipped++
			slog.Info("driver already exists, skipping", "email", seed.Email)
		default:
			return created, skipped, createErr
		}
	}
	return created, skipped, nil
}

func seedVehicles(ctx context.Context, gdb *gorm.DB) (created, skipped int, err error) {
	svc := vehicles.NewService(vehicles.NewRepository(gdb))

	for _, seed := range vehicleSeeds {
		model := seed.Model
		_, createErr := svc.Create(ctx, vehicles.CreateInput{
			RegistrationNumber: seed.Registration,
			Model:              &model,
			Capacity:           seed.Capacity,
			Status:             vehicles.StatusAvailable,
		})
		switch {
		case createErr == nil:
			created++
			slog.Info("seeded vehicle", "registration", seed.Registration, "model", seed.Model)
		case errors.Is(createErr, vehicles.ErrRegistrationTaken), db.IsUniqueViolation(createErr):
			skipped++
			slog.Info("vehicle already exists, skipping", "registration", seed.Registration)
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
