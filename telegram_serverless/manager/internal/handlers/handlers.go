package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/manager/internal/bot"
	"github.com/uchebnick/telegram-serverless/manager/internal/models"
	"go.uber.org/zap"
)

type Handlers struct {
	botService *bot.Service
	logger     *zap.SugaredLogger
}

func NewHandlers(botService *bot.Service, logger *zap.SugaredLogger) *Handlers {
	return &Handlers{
		botService: botService,
		logger:     logger,
	}
}

// CreateBot handles POST /bots
func (h *Handlers) CreateBot(c *fiber.Ctx) error {
	var req models.CreateBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	response, err := h.botService.CreateBot(c.UserContext(), &req)
	if err != nil {
		h.logger.Errorw("failed to create bot", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// GetBot handles GET /bots/{bot_id}
func (h *Handlers) GetBot(c *fiber.Ctx) error {
	botID := c.Params("bot_id")

	response, err := h.botService.GetBot(c.UserContext(), botID)
	if err != nil {
		h.logger.Errorw("failed to get bot", "bot_id", botID, "error", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bot not found"})
	}

	return c.JSON(response)
}

// DeleteBot handles DELETE /bots/{bot_id}
func (h *Handlers) DeleteBot(c *fiber.Ctx) error {
	botID := c.Params("bot_id")

	if err := h.botService.DeleteBot(c.UserContext(), botID); err != nil {
		h.logger.Errorw("failed to delete bot", "bot_id", botID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "bot deleted"})
}

// UpdateReplicas handles PATCH /bots/{bot_id}/replicas
func (h *Handlers) UpdateReplicas(c *fiber.Ctx) error {
	botID := c.Params("bot_id")

	var req models.UpdateReplicasRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.botService.UpdateReplicas(c.UserContext(), botID, &req); err != nil {
		h.logger.Errorw("failed to update replicas", "bot_id", botID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "replicas updated"})
}

// ListBots handles GET /bots
func (h *Handlers) ListBots(c *fiber.Ctx) error {
	bots, err := h.botService.ListBots(c.UserContext())
	if err != nil {
		h.logger.Errorw("failed to list bots", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list bots"})
	}

	return c.JSON(bots)
}
