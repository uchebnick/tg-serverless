package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/webhook"
	"go.uber.org/zap"
)

type Server struct {
	app            *fiber.App
	webhookHandler *webhook.Handler
	logger         *zap.SugaredLogger
}

func NewServer(webhookHandler *webhook.Handler, logger *zap.SugaredLogger) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * 1e9,
		WriteTimeout: 15 * 1e9,
		IdleTimeout:  60 * 1e9,
	})

	return &Server{
		app:            app,
		webhookHandler: webhookHandler,
		logger:         logger,
	}
}

func (s *Server) setupRoutes() {
	s.app.Post("/webhook/:bot_token", s.webhookHandler.HandleWebhook)
	s.app.Get("/health", healthHandler)
	s.app.Get("/ready", readyHandler)
}

func (s *Server) Start(port string) error {
	s.setupRoutes()
	s.logger.Info("http server listening", zap.String("port", port))
	return s.app.Listen(":" + port)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
