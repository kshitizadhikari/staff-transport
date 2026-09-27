package redis

import (
	"context"
	"fmt"

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
