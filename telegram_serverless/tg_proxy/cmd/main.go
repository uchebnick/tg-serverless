package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/config"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/handlers"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/kafka"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/routes"
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

	producer := kafka.NewProducer(cfg.KafkaBrokers, logger, cfg.OutgoingTopic)
	defer producer.Close()

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, cfg.BotID, cfg.IncomingTopic, logger)
	defer consumer.Close()

	apiHandlers := handlers.NewHandlers(producer, consumer, logger)
	server := routes.NewServer(apiHandlers, logger)

	metricsSrv := newMetricsServer(cfg.MetricsPort, logger)

	go func() {
		if err := server.Start(cfg.Port); err != nil {
			logger.Fatalf("failed to start server: %v", err)
		}
	}()

	go func() {
		if err := metricsSrv.Listen(":" + cfg.MetricsPort); err != nil {
			logger.Error("metrics server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	if err := server.Shutdown(); err != nil {
		logger.Fatalf("failed to shutdown server: %v", err)
	}
	if err := metricsSrv.Shutdown(); err != nil {
		logger.Error("metrics server shutdown error", zap.Error(err))
	}

	logger.Info("server gracefully stopped")
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
