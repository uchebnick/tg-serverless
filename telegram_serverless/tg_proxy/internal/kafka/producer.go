package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Producer struct {
	writer *kafka.Writer
	logger *zap.SugaredLogger
	topic  string
}

func NewProducer(brokers []string, logger *zap.SugaredLogger, topic string) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
		Async:        true,
		Compression:  kafka.Snappy,
		Logger:       kafka.LoggerFunc(logger.Infof),
		ErrorLogger:  kafka.LoggerFunc(logger.Errorf),
	}

	return &Producer{
		topic:  topic,
		writer: writer,
		logger: logger,
	}
}

func (p *Producer) PublishMessage(ctx context.Context, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	msg := kafka.Message{
		Topic: p.topic,
		Value: data,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
