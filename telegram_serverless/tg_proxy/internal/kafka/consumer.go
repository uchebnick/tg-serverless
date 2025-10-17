package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Consumer struct {
	reader *kafka.Reader
	logger *zap.SugaredLogger
	topic  string
}

func NewConsumer(brokers []string, groupID string, topic string, logger *zap.SugaredLogger) *Consumer {
	config := kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	}

	if groupID != "" {
		config.GroupID = groupID
		config.CommitInterval = time.Second
		config.StartOffset = kafka.LastOffset
	} else {
		config.StartOffset = kafka.FirstOffset
	}

	reader := kafka.NewReader(config)

	return &Consumer{
		reader: reader,
		logger: logger,
		topic:  topic,
	}
}

func (c *Consumer) ReadAllMessages(ctx context.Context) ([]kafka.Message, error) {
	var messages []kafka.Message

	conn, err := kafka.DialLeader(ctx, "tcp", c.reader.Config().Brokers[0], c.topic, 0)
	if err != nil {
		return nil, err
	}

	_, highWaterMark, err := conn.ReadOffsets()
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn.Close()

	c.logger.Infow("reading messages from topic",
		"topic", c.topic,
		"high_water_mark", highWaterMark)

	for {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := c.reader.ReadMessage(readCtx)
		cancel()

		if err != nil {
			if err == context.DeadlineExceeded {
				c.logger.Info("reached end of topic (timeout)")
				break
			}
			if err == context.Canceled {
				break
			}
			c.logger.Errorw("failed to read message", "error", err)
			break
		}

		messages = append(messages, msg)

		if msg.Offset >= highWaterMark-1 {
			c.logger.Info("reached end of topic (high water mark)")
			break
		}
	}

	c.logger.Infow("finished reading messages", "count", len(messages))
	return messages, nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
