package webhook

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/kafka"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/models"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/storage"
	"go.uber.org/zap"
)

type Handler struct {
	storage  *storage.RedisStorage
	producer *kafka.Producer
	logger   *zap.SugaredLogger
}

func NewHandler(storage *storage.RedisStorage, producer *kafka.Producer, logger *zap.SugaredLogger) *Handler {
	return &Handler{
		storage:  storage,
		producer: producer,
		logger:   logger,
	}
}

// HandleWebhook processes incoming webhook from Telegram
// POST /webhook/{bot_token}
func (h *Handler) HandleWebhook(c *fiber.Ctx) error {
	botToken := c.Params("bot_token")

	if botToken == "" {
		return c.Status(fiber.StatusBadRequest).SendString("bot_token is required")
	}

	var update models.TelegramUpdate
	if err := c.BodyParser(&update); err != nil {
		h.logger.Error("failed to unmarshal update", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("invalid json")
	}

	ctx := context.Background()
	botID, err := h.storage.GetBotIDByToken(ctx, botToken)
	if err != nil {
		h.logger.Error("failed to get bot_id",
			zap.String("bot_token", maskToken(botToken)),
			zap.Error(err))
		return c.Status(fiber.StatusNotFound).SendString("bot not found")
	}

	incomingMsg := models.IncomingMessage{
		BotID:  botID,
		Update: update,
	}

	topic := fmt.Sprintf("bot_%s_incoming", botID)
	key := fmt.Sprintf("%d", update.UpdateID)

	if err := h.producer.PublishMessage(ctx, topic, key, incomingMsg); err != nil {
		h.logger.Error("failed to publish to kafka",
			zap.String("bot_id", botID),
			zap.String("topic", topic),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("internal error")
	}

	h.logger.Debug("webhook processed",
		zap.String("bot_id", botID),
		zap.Int("update_id", update.UpdateID))

	return c.SendString("ok")
}

func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return token[:5] + "***" + token[len(token)-5:]
}
