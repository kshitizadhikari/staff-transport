package redis

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"staff-transport/internal/config"
)

func Open(ctx context.Context, cfg *config.Config) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

func Ping(ctx context.Context, client *goredis.Client) error {
	return client.Ping(ctx).Err()
}

// AsynqOpt builds an Asynq Redis connection option from REDIS_URL.
func AsynqOpt(rawURL string) (asynq.RedisClientOpt, error) {
	opts, err := goredis.ParseURL(rawURL)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	return asynq.RedisClientOpt{
		Addr:     opts.Addr,
		Username: opts.Username,
		Password: opts.Password,
		DB:       opts.DB,
	}, nil
}
