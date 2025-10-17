package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uchebnick/telegram-serverless/manager/internal/bot"
	"github.com/uchebnick/telegram-serverless/manager/internal/config"
	"github.com/uchebnick/telegram-serverless/manager/internal/handlers"
	"github.com/uchebnick/telegram-serverless/manager/internal/kafka"
	"github.com/uchebnick/telegram-serverless/manager/internal/kubernetes"
	"github.com/uchebnick/telegram-serverless/manager/internal/routes"
	"github.com/uchebnick/telegram-serverless/manager/internal/storage"
	"github.com/uchebnick/telegram-serverless/manager/internal/telegram"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Infow("starting manager",
		"port", cfg.Port,
		"kafka_brokers", cfg.KafkaBrokers,
		"worker_namespace", cfg.WorkerNamespace)

	ctx := context.Background()

	redisStorage, err := storage.NewRedisStorage(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Fatalw("failed to initialize redis", "error", err)
	}
	defer redisStorage.Close()

	kafkaAdmin := kafka.NewAdmin(cfg.KafkaBrokers, logger)

	k8sClient, err := kubernetes.NewClient(cfg.WorkerNamespace, cfg.SidecarImage, logger)
	if err != nil {
		logger.Fatalw("failed to initialize kubernetes client", "error", err)
	}

	if err := k8sClient.Ping(ctx); err != nil {
		logger.Fatalw("failed to connect to kubernetes", "error", err)
	}

	tgClient := telegram.NewClient(cfg.GatewayURL, logger)

	kafkaBrokersStr := strings.Join(cfg.KafkaBrokers, ",")
	botService := bot.NewService(
		redisStorage,
		kafkaAdmin,
		k8sClient,
		tgClient,
		cfg.GatewayURL,
		kafkaBrokersStr,
		cfg.TlsCaSecretName,
		logger,
	)

	apiHandlers := handlers.NewHandlers(botService, logger)

	apiServer := routes.NewServer(apiHandlers, logger)

	metricsSrv := newMetricsServer(cfg.MetricsPort, logger)

	go func() {
		if err := apiServer.Start(cfg.Port); err != nil {
			logger.Fatalw("handlers server error", "error", err)
		}
	}()

	go func() {
		if err := metricsSrv.Listen(":" + cfg.MetricsPort); err != nil {
			logger.Fatalw("metrics server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully...")

	if err := apiServer.Shutdown(); err != nil {
		logger.Errorw("handlers server shutdown error", "error", err)
	}

	if err := metricsSrv.Shutdown(); err != nil {
		logger.Errorw("metrics server shutdown error", "error", err)
	}

	logger.Info("manager stopped")
}

func initLogger(level string) (*zap.SugaredLogger, error) {
	cfg := zap.NewProductionConfig()

	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

func newMetricsServer(port string, logger *zap.SugaredLogger) *fiber.App {
	app := fiber.New()
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	logger.Info("metrics server listening", zap.String("port", port))
	return app
}
