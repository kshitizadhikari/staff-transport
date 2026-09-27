package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"staff-transport/internal/auth"
	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/dispatch"
	"staff-transport/internal/drivers"
	"staff-transport/internal/events"
	"staff-transport/internal/locations"
	"staff-transport/internal/maps"
	"staff-transport/internal/notifications"
	"staff-transport/internal/redis"
	"staff-transport/internal/server"
	"staff-transport/internal/staff"
	"staff-transport/internal/trips"
	"staff-transport/internal/users"
	"staff-transport/internal/vehicles"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gdb, err := db.Open(ctx, cfg)
	if err != nil {
		return err
	}
	rdb, err := redis.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer rdb.Close()

	geocoder := maps.NewMapboxGeocoder(cfg.MapboxAccessToken, cfg.MapboxCountry)

	staffSvc := staff.NewService(staff.NewRepository(gdb), geocoder)
	driverSvc := drivers.NewService(drivers.NewRepository(gdb))
	vehicleSvc := vehicles.NewService(vehicles.NewRepository(gdb))

	asynqOpt, err := redis.AsynqOpt(cfg.RedisURL)
	if err != nil {
		return err
	}
	asynqClient := asynq.NewClient(asynqOpt)
	defer asynqClient.Close()

	notifSvc := notifications.NewService(
		notifications.NewRepository(gdb),
		notifications.NewExpoSender(cfg.ExpoAccessToken),
		asynqClient,
	)
	tripSvc := trips.NewService(trips.NewRepository(gdb), driverSvc, vehicleSvc, staffSvc, notifSvc, geocoder, cfg.OrgTimezone)
	locationSvc := locations.NewService(locations.NewRepository(gdb), tripSvc)
	dispatchSvc := dispatch.NewService(dispatch.NewRepository(gdb), tripSvc)
	eventSvc := events.NewService(events.NewRepository(gdb), staffSvc, geocoder)

	srv := server.New(server.Dependencies{
		Config:        cfg,
		DB:            gdb,
		Redis:         rdb,
		Auth:          auth.NewService(cfg, rdb),
		Users:         users.NewService(users.NewRepository(gdb)),
		Staff:         staffSvc,
		Drivers:       driverSvc,
		Vehicles:      vehicleSvc,
		Trips:         tripSvc,
		Locations:     locationSvc,
		Notifications: notifSvc,
		Dispatch:      dispatchSvc,
		Events:        eventSvc,
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", httpServer.Addr, "env", cfg.AppEnv)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
