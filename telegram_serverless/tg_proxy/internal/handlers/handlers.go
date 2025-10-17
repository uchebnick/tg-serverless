package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/kafka"
	"github.com/uchebnick/telegram-serverless/tg_proxy/internal/models"
	"go.uber.org/zap"
)

type Handlers struct {
	producer *kafka.Producer
	consumer *kafka.Consumer
	logger   *zap.SugaredLogger
}

func NewHandlers(producer *kafka.Producer, consumer *kafka.Consumer, logger *zap.SugaredLogger) *Handlers {
	return &Handlers{
		producer: producer,
		consumer: consumer,
		logger:   logger,
	}
}

// GetUpdates handles GET bot:token/getUpdates
// Возвращает все updates из Kafka топика
func (h *Handlers) GetUpdates(c *fiber.Ctx) error {
	messages, err := h.consumer.ReadAllMessages(c.UserContext())
	if err != nil {
		h.logger.Errorw("failed to read messages", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"ok":          false,
			"error_code":  500,
			"description": "Failed to read messages",
		})
	}

	updates := make([]models.TelegramUpdate, 0, len(messages))
	for _, msg := range messages {
		var incoming models.TelegramUpdate
		if err := json.Unmarshal(msg.Value, &incoming); err != nil {
			h.logger.Errorw("failed to parse message",
				"error", err,
				"offset", msg.Offset)
			continue
		}
		updates = append(updates, incoming)
	}

	h.logger.Infow("processed messages",
		"total", len(messages),
		"parsed", len(updates),
		"failed", len(messages)-len(updates))

	return c.JSON(fiber.Map{
		"ok":     true,
		"result": updates,
	})
}

// GetMe handles GET bot:token/getMe
func (h *Handlers) GetMe(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"ok": true})
}

// CallMethod handles all POST methods (sendMessage, sendPhoto, etc.)
// POST bot:token/method:
func (h *Handlers) CallMethod(c *fiber.Ctx) error {
	method := c.Params("method")
	botToken := c.Params("token")

	if method == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ok":          false,
			"error_code":  400,
			"description": "Method name is required",
		})
	}

	var params map[string]interface{}
	if err := c.BodyParser(&params); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"ok":          false,
			"error_code":  400,
			"description": "Invalid JSON",
		})
	}

	h.logger.Infow("calling telegram method",
		"method", method,
		"params", params)

	outgoing := models.OutgoingCommand{
		Method:   method,
		Params:   params,
		BotToken: botToken,
	}
	if err := h.producer.PublishMessage(c.UserContext(), outgoing); err != nil {
		h.logger.Errorw("failed to publish message", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"ok":          false,
			"error_code":  500,
			"description": "Failed to publish message",
		})
	}

	return c.JSON(fiber.Map{"ok": true})
}
