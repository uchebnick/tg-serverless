package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/manager/internal/handlers"
	"go.uber.org/zap"
)

type Server struct {
	app      *fiber.App
	logger   *zap.SugaredLogger
	handlers *handlers.Handlers
}

func NewServer(handlers *handlers.Handlers, logger *zap.SugaredLogger) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * 1e9,
		WriteTimeout: 15 * 1e9,
		IdleTimeout:  60 * 1e9,
	})

	return &Server{
		app:      app,
		logger:   logger,
		handlers: handlers,
	}
}

func (s *Server) setupRoutes() {
	s.app.Post("/bots", s.handlers.CreateBot)
	s.app.Get("/bots", s.handlers.ListBots)
	s.app.Get("/bots/:bot_id", s.handlers.GetBot)
	s.app.Delete("/bots/:bot_id", s.handlers.DeleteBot)
	s.app.Patch("/bots/:bot_id/replicas", s.handlers.UpdateReplicas)

	s.app.Get("/health", healthHandler)
	s.app.Get("/ready", readyHandler)
}

func (s *Server) Start(port string) error {
	s.setupRoutes()
	s.logger.Infow("starting handlers server", "port", port)
	return s.app.Listen(":" + port)
}

func (s *Server) Shutdown() error {
	s.logger.Info("shutting down handlers server")
	return s.app.Shutdown()
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
