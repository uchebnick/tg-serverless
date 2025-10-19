package cmd

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	KafkaBrokers       []string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	GatewayURL         string
	WorkerNamespace    string
	MetricsPort        string
	LogLevel           string
	DefaultWorkerImage string
	SidecarImage       string
	TlsCaSecretName    string
}

func Load() *Config {
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")

	return &Config{
		Port:               getEnv("PORT", "8080"),
		KafkaBrokers:       kafkaBrokers,
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            redisDB,
		GatewayURL:         getEnv("GATEWAY_URL", "http://tg-gateway:8080"),
		WorkerNamespace:    getEnv("WORKER_NAMESPACE", "default"),
		MetricsPort:        getEnv("METRICS_PORT", "9090"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		DefaultWorkerImage: getEnv("DEFAULT_WORKER_IMAGE", ""),
		SidecarImage:       getEnv("SIDECAR_IMAGE", "your-registry/sidecar:latest"),
		TlsCaSecretName:    getEnv("TLS_CA_SECRET_NAME", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
