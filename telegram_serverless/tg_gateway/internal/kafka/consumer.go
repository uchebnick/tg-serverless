package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Consumer struct {
	reader *kafka.Reader
	logger *zap.SugaredLogger
}

func NewConsumer(brokers []string, groupID string, topics []string, logger *zap.SugaredLogger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{
		reader: reader,
		logger: logger,
	}
}

func (c *Consumer) ConsumeMessages(ctx context.Context, handler func(kafka.Message) error) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer context cancelled, shutting down")
			return ctx.Err()
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				c.logger.Error("failed to fetch message", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			c.logger.Debug("received message",
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset))

			if err := handler(msg); err != nil {
				c.logger.Error("failed to process message",
					zap.String("topic", msg.Topic),
					zap.Int64("offset", msg.Offset),
					zap.Error(err))
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error("failed to commit message", zap.Error(err))
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

type TopicManager struct {
	brokers []string
	logger  *zap.SugaredLogger
}

func NewTopicManager(brokers []string, logger *zap.SugaredLogger) *TopicManager {
	return &TopicManager{
		brokers: brokers,
		logger:  logger,
	}
}

func (tm *TopicManager) GetAllOutgoingTopics(ctx context.Context) ([]string, error) {
	conn, err := kafka.Dial("tcp", tm.brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	topicMap := make(map[string]bool)
	for _, p := range partitions {
		topic := p.Topic
		if len(topic) > 8 && topic[:4] == "bot_" && topic[len(topic)-9:] == "_outgoing" {
			topicMap[topic] = true
		}
	}

	topics := make([]string, 0, len(topicMap))
	for topic := range topicMap {
		topics = append(topics, topic)
	}

	tm.logger.Info("discovered outgoing topics", zap.Strings("topics", topics))
	return topics, nil
}
