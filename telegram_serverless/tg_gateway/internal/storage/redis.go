package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

func (r *RedisStorage) GetBotIDByToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("bot:token:%s", token)
	botID, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get bot_id for token: %w", err)
	}
	return botID, nil
}

func (r *RedisStorage) GetOutgoingTopic(ctx context.Context, botID string) (string, error) {
	return fmt.Sprintf("bot_%s_outgoing", botID), nil
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}
