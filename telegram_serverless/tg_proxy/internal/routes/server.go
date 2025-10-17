package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/handlers"
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
func (s *Server) SetupRoutes() {
	s.app.Get("/bot:token/getMe", s.handlers.GetMe)
	s.app.Get("/bot:token/getUpdates", s.handlers.GetUpdates)
	s.app.Post("/bot:token/:method", s.handlers.CallMethod)

	s.app.Get("/health", healthHandler)
	s.app.Get("/ready", readyHandler)
}

func (s *Server) Start(port string) error {
	s.SetupRoutes()
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
