package cache

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/roypratim/skyhigh/internal/config"
)

// NewRedisClient creates and validates a Redis client connection.
func NewRedisClient(cfg *config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
		DB:   0,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}

	log.Println("Redis connected")
	return client
}
