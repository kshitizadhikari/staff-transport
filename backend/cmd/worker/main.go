package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"staff-transport/internal/config"
	"staff-transport/internal/db"
	"staff-transport/internal/notifications"
	"staff-transport/internal/redis"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited with error", "error", err)
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

	redisOpt, err := redis.AsynqOpt(cfg.RedisURL)
	if err != nil {
		return err
	}

	notifier := notifications.NewService(
		notifications.NewRepository(gdb),
		notifications.NewExpoSender(cfg.ExpoAccessToken),
		nil,
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(notifications.TypeSendNotification, notifier.HandleSend)

	worker := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 5,
		Queues:      map[string]int{"default": 1},
	})

	errCh := make(chan error, 1)
	go func() {
		slog.Info("worker starting", "env", cfg.AppEnv)
		if err := worker.Run(mux); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		worker.Shutdown()
		return nil
	}
}
