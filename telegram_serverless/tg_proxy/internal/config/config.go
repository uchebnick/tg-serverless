package config

import (
	"os"
)

type Config struct {
	Port          string
	KafkaBrokers  []string
	MetricsPort   string
	LogLevel      string
	BotToken      string
	IncomingTopic string
	OutgoingTopic string
}

func Load() *Config {

	return &Config{
		Port:          getEnv("PORT", "8080"),
		KafkaBrokers:  []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		MetricsPort:   getEnv("METRICS_PORT", "9090"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		BotToken:      getEnv("BOT_TOKEN", ""),
		IncomingTopic: getEnv("INCOMING_TOPIC", ""),
		OutgoingTopic: getEnv("OUTGOING_TOPIC", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
