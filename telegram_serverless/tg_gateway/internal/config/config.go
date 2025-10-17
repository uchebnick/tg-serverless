package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	KafkaBrokers         []string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	TelegramAPIURL       string
	MetricsPort          string
	LogLevel             string
	OutgoingTopicPattern string
}

func Load() *Config {
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	return &Config{
		Port:                 getEnv("PORT", "8080"),
		KafkaBrokers:         []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              redisDB,
		TelegramAPIURL:       getEnv("TELEGRAM_API_URL", "https://handlers.telegram.org"),
		MetricsPort:          getEnv("METRICS_PORT", "9090"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		OutgoingTopicPattern: getEnv("OUTGOING_TOPIC_PATTERN", "bot_.*_outgoing"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
