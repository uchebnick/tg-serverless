package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Admin struct {
	brokers []string
	logger  *zap.SugaredLogger
}

func NewAdmin(brokers []string, logger *zap.SugaredLogger) *Admin {
	return &Admin{
		brokers: brokers,
		logger:  logger,
	}
}

func (a *Admin) CreateTopics(ctx context.Context, botID string) error {
	conn, err := kafka.Dial("tcp", a.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to dial controller: %w", err)
	}
	defer controllerConn.Close()

	incomingTopic := fmt.Sprintf("bot_%s_incoming", botID)
	outgoingTopic := fmt.Sprintf("bot_%s_outgoing", botID)

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             incomingTopic,
			NumPartitions:     3,
			ReplicationFactor: 2,
		},
		{
			Topic:             outgoingTopic,
			NumPartitions:     3,
			ReplicationFactor: 2,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("failed to create topics: %w", err)
	}

	a.logger.Infow("kafka topics created",
		"bot_id", botID,
		"incoming_topic", incomingTopic,
		"outgoing_topic", outgoingTopic)

	return nil
}

func (a *Admin) DeleteTopics(ctx context.Context, botID string) error {
	conn, err := kafka.Dial("tcp", a.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to dial controller: %w", err)
	}
	defer controllerConn.Close()

	incomingTopic := fmt.Sprintf("bot_%s_incoming", botID)
	outgoingTopic := fmt.Sprintf("bot_%s_outgoing", botID)

	err = controllerConn.DeleteTopics(incomingTopic, outgoingTopic)
	if err != nil {
		return fmt.Errorf("failed to delete topics: %w", err)
	}

	a.logger.Infow("kafka topics deleted",
		"bot_id", botID,
		"incoming_topic", incomingTopic,
		"outgoing_topic", outgoingTopic)

	return nil
}
