package outgoing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	kafkapkg "github.com/uchebnick/telegram-serverless/tg_gateway/internal/kafka"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/models"
	"github.com/uchebnick/telegram-serverless/tg_gateway/internal/telegram"
	"go.uber.org/zap"
)

type Processor struct {
	brokers        []string
	groupID        string
	telegramClient *telegram.Client
	logger         *zap.SugaredLogger
	consumers      map[string]*kafkapkg.Consumer
	mu             sync.RWMutex
	topicManager   *kafkapkg.TopicManager
	stopChan       chan struct{}
}

func NewProcessor(brokers []string, groupID string, telegramClient *telegram.Client, logger *zap.SugaredLogger) *Processor {
	return &Processor{
		brokers:        brokers,
		groupID:        groupID,
		telegramClient: telegramClient,
		logger:         logger,
		consumers:      make(map[string]*kafkapkg.Consumer),
		topicManager:   kafkapkg.NewTopicManager(brokers, logger),
		stopChan:       make(chan struct{}),
	}
}

func (p *Processor) Start(ctx context.Context) error {
	if err := p.discoverAndSubscribe(ctx); err != nil {
		return fmt.Errorf("failed to discover topics: %w", err)
	}

	go p.periodicTopicDiscovery(ctx)

	<-ctx.Done()
	return nil
}

func (p *Processor) periodicTopicDiscovery(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.discoverAndSubscribe(ctx); err != nil {
				p.logger.Error("failed to rediscover topics", zap.Error(err))
			}
		}
	}
}

func (p *Processor) discoverAndSubscribe(ctx context.Context) error {
	topics, err := p.topicManager.GetAllOutgoingTopics(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, topic := range topics {
		if _, exists := p.consumers[topic]; !exists {
			p.logger.Info("subscribing to new topic", zap.String("topic", topic))

			consumer := kafkapkg.NewConsumer(p.brokers, p.groupID, []string{topic}, p.logger)
			p.consumers[topic] = consumer

			go func(topic string, consumer *kafkapkg.Consumer) {
				if err := consumer.ConsumeMessages(ctx, p.handleMessage); err != nil {
					p.logger.Error("consumer error", zap.String("topic", topic), zap.Error(err))
				}
			}(topic, consumer)
		}
	}

	return nil
}

func (p *Processor) handleMessage(msg kafka.Message) error {
	var cmd models.OutgoingCommand
	if err := json.Unmarshal(msg.Value, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	p.logger.Debug("processing outgoing command",
		zap.String("method", cmd.Method),
		zap.String("topic", msg.Topic))

	result, err := p.telegramClient.CallMethod(cmd.BotToken, cmd.Method, cmd.Params)
	if err != nil {
		p.logger.Error("failed to call telegram handlers",
			zap.String("method", cmd.Method),
			zap.Error(err))
		return err
	}

	p.logger.Debug("telegram handlers call successful",
		zap.String("method", cmd.Method),
		zap.ByteString("result", result))

	return nil
}

func (p *Processor) Stop() {
	close(p.stopChan)

	p.mu.Lock()
	defer p.mu.Unlock()

	for topic, consumer := range p.consumers {
		p.logger.Info("closing consumer", zap.String("topic", topic))
		if err := consumer.Close(); err != nil {
			p.logger.Error("failed to close consumer", zap.String("topic", topic), zap.Error(err))
		}
	}
}
