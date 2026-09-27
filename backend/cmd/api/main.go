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

	"staff-transport/internal/auth"
	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/drivers"
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

	staffSvc := staff.NewService(staff.NewRepository(gdb))
	driverSvc := drivers.NewService(drivers.NewRepository(gdb))
	vehicleSvc := vehicles.NewService(vehicles.NewRepository(gdb))
	tripSvc := trips.NewService(trips.NewRepository(gdb), driverSvc, vehicleSvc, staffSvc, cfg.OrgTimezone)

	srv := server.New(server.Dependencies{
		Config:   cfg,
		DB:       gdb,
		Redis:    rdb,
		Auth:     auth.NewService(cfg, rdb),
		Users:    users.NewService(users.NewRepository(gdb)),
		Staff:    staffSvc,
		Drivers:  driverSvc,
		Vehicles: vehicleSvc,
		Trips:    tripSvc,
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
