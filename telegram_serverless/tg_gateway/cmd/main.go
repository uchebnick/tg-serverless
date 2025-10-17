package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/config"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/kafka"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/outgoing"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/routes"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/storage"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/telegram"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/webhook"
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

	logger.Info("starting tg_gateway",
		zap.String("port", cfg.Port),
		zap.Strings("kafka_brokers", cfg.KafkaBrokers))

	redisStorage, err := storage.NewRedisStorage(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Fatal("failed to initialize redis", zap.Error(err))
	}
	defer redisStorage.Close()

	producer := kafka.NewProducer(cfg.KafkaBrokers, logger)
	defer producer.Close()

	tgClient := telegram.NewClient(cfg.TelegramAPIURL, logger)

	webhookHandler := webhook.NewHandler(redisStorage, producer, logger)

	outgoingProcessor := outgoing.NewProcessor(cfg.KafkaBrokers, "tg-gateway-outgoing", tgClient, logger)

	server := routes.NewServer(webhookHandler, logger)
	metricsServer := newMetricsServer(cfg.MetricsPort, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := outgoingProcessor.Start(ctx); err != nil {
			logger.Error("outgoing processor error", zap.Error(err))
		}
	}()

	go func() {
		if err := server.Start(cfg.Port); err != nil {
			logger.Error("http server error", zap.Error(err))
		}
	}()
	go func() {
		if err := metricsServer.Listen(":" + cfg.MetricsPort); err != nil {
			logger.Error("metrics server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully...")

	outgoingProcessor.Stop()

	if err := server.Shutdown(); err != nil {
		logger.Error("http server shutdown error", zap.Error(err))
	}
	if err := metricsServer.Shutdown(); err != nil {
		logger.Error("metrics server shutdown error", zap.Error(err))
	}

	logger.Info("tg_gateway stopped")
}

func newMetricsServer(port string, logger *zap.SugaredLogger) *fiber.App {
	app := fiber.New()
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	logger.Info("metrics server listening", zap.String("port", port))
	return app
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
