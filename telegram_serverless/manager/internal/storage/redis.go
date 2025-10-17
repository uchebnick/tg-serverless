package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/uchebnick/telegram-serverless/manager/internal/models"
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

func (r *RedisStorage) SaveBot(ctx context.Context, botConfig *models.BotConfig) error {
	data, err := json.Marshal(botConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal bot config: %w", err)
	}

	configKey := fmt.Sprintf("bot:config:%s", botConfig.BotID)
	if err := r.client.Set(ctx, configKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save bot config: %w", err)
	}

	tokenKey := fmt.Sprintf("bot:token:%s", botConfig.BotToken)
	if err := r.client.Set(ctx, tokenKey, botConfig.BotID, 0).Err(); err != nil {
		return fmt.Errorf("failed to save token mapping: %w", err)
	}

	if err := r.client.SAdd(ctx, "bots:all", botConfig.BotID).Err(); err != nil {
		return fmt.Errorf("failed to add bot to list: %w", err)
	}

	return nil
}

// GetBot retrieves bot configuration from Redis
func (r *RedisStorage) GetBot(ctx context.Context, botID string) (*models.BotConfig, error) {
	key := fmt.Sprintf("bot:config:%s", botID)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("bot not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bot config: %w", err)
	}

	var botConfig models.BotConfig
	if err := json.Unmarshal([]byte(data), &botConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bot config: %w", err)
	}

	return &botConfig, nil
}

// DeleteBot removes bot configuration from Redis
func (r *RedisStorage) DeleteBot(ctx context.Context, botID string, botToken string) error {
	configKey := fmt.Sprintf("bot:config:%s", botID)
	tokenKey := fmt.Sprintf("bot:token:%s", botToken)

	pipe := r.client.Pipeline()
	pipe.Del(ctx, configKey)
	pipe.Del(ctx, tokenKey)
	pipe.SRem(ctx, "bots:all", botID)

	_, err := pipe.Exec(ctx)
	return err
}

// ListBots retrieves all bot IDs
func (r *RedisStorage) ListBots(ctx context.Context) ([]string, error) {
	botIDs, err := r.client.SMembers(ctx, "bots:all").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list bots: %w", err)
	}
	return botIDs, nil
}

func (r *RedisStorage) UpdateBotStatus(ctx context.Context, botID, status string) error {
	botConfig, err := r.GetBot(ctx, botID)
	if err != nil {
		return err
	}

	botConfig.Status = status
	return r.SaveBot(ctx, botConfig)
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}
