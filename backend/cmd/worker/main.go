package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"staff-transport/internal/config"
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

	redisOpt, err := redisConnOpt(cfg.RedisURL)
	if err != nil {
		return err
	}

	// Background task handlers are registered here as modules are implemented:
	//   mux.HandleFunc(tasks.TypeSendNotification, notifications.HandleSend)
	//   mux.HandleFunc(tasks.TypeGenerateRecurringTrips, dispatch.HandleGenerate)
	mux := asynq.NewServeMux()

	worker := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 5,
		Queues:      map[string]int{"default": 1},
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

func redisConnOpt(rawURL string) (asynq.RedisConnOpt, error) {
	opts, err := goredis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	return asynq.RedisClientOpt{
		Addr:     opts.Addr,
		Username: opts.Username,
		Password: opts.Password,
		DB:       opts.DB,
	}, nil
}
