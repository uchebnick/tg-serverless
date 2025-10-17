package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/uchebnick/telegram-serverless/manager/internal/kafka"
	"github.com/uchebnick/telegram-serverless/manager/internal/kubernetes"
	"github.com/uchebnick/telegram-serverless/manager/internal/models"
	"github.com/uchebnick/telegram-serverless/manager/internal/storage"
	"github.com/uchebnick/telegram-serverless/manager/internal/telegram"
	"go.uber.org/zap"
)

type Service struct {
	storage         *storage.RedisStorage
	kafkaAdmin      *kafka.Admin
	k8sClient       *kubernetes.Client
	tgClient        *telegram.Client
	gatewayURL      string
	kafkaBrokers    string
	tlsCaSecretName string
	logger          *zap.SugaredLogger
}

func NewService(
	storage *storage.RedisStorage,
	kafkaAdmin *kafka.Admin,
	k8sClient *kubernetes.Client,
	tgClient *telegram.Client,
	gatewayURL string,
	kafkaBrokers string,
	tlsCaSecretName string,
	logger *zap.SugaredLogger,
) *Service {
	return &Service{
		storage:         storage,
		kafkaAdmin:      kafkaAdmin,
		k8sClient:       k8sClient,
		tgClient:        tgClient,
		gatewayURL:      gatewayURL,
		kafkaBrokers:    kafkaBrokers,
		tlsCaSecretName: tlsCaSecretName,
		logger:          logger,
	}
}

func (s *Service) CreateBot(ctx context.Context, req *models.CreateBotRequest) (*models.CreateBotResponse, error) {
	botID := s.generateBotID()

	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	botConfig := &models.BotConfig{
		BotID:       botID,
		BotToken:    req.BotToken,
		BotName:     req.BotName,
		WorkerImage: req.WorkerImage,
		MinReplicas: req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
		EnvVars:     req.EnvVars,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      "creating",
	}

	if err := s.storage.SaveBot(ctx, botConfig); err != nil {
		return nil, fmt.Errorf("failed to save bot config: %w", err)
	}

	s.logger.Infow("creating kafka topics", "bot_id", botID)
	if err := s.kafkaAdmin.CreateTopics(ctx, botID); err != nil {
		s.updateBotStatus(ctx, botID, "failed")
		return nil, fmt.Errorf("failed to create kafka topics: %w", err)
	}

	// Create Kubernetes resources (Secret, Deployment)
	s.logger.Infow("creating kubernetes resources", "bot_id", botID)
	if err := s.k8sClient.CreateBotResources(ctx, botConfig, s.kafkaBrokers); err != nil {
		s.updateBotStatus(ctx, botID, "failed")
		return nil, fmt.Errorf("failed to create k8s resources: %w", err)
	}

	s.logger.Infow("creating keda scaledobject", "bot_id", botID)
	if err := s.k8sClient.CreateScaledObject(ctx, botConfig, s.kafkaBrokers); err != nil {
		s.updateBotStatus(ctx, botID, "failed")
		return nil, fmt.Errorf("failed to create scaledobject: %w", err)
	}

	webhookURL := fmt.Sprintf("%s/webhook/%s", s.gatewayURL, req.BotToken)
	s.logger.Infow("setting telegram webhook", "bot_id", botID, "webhook_url", webhookURL)
	if strings.HasPrefix(s.gatewayURL, "https://") {
		var caCert []byte
		var err error
		if s.tlsCaSecretName != "" {
			caCert, err = s.k8sClient.GetSecret(ctx, s.tlsCaSecretName)
			if err != nil {
				s.logger.Errorw("failed to get ca certificate from secret", "error", err, "secret_name", s.tlsCaSecretName)
				// Можно продолжить без сертификата, но лучше залогировать ошибку
			}
		}

		if err := s.tgClient.SetWebhook(req.BotToken, webhookURL, caCert); err != nil {
			s.logger.Errorw("failed to set webhook", "error", err)
		}
	} else {
		s.logger.Warnw("gateway_url is not a public https url, skipping setting webhook. Use this url for local testing", "webhook_url", webhookURL)
	}

	if err := s.updateBotStatus(ctx, botID, "running"); err != nil {
		s.logger.Errorw("failed to update bot status", "error", err)
	}

	response := &models.CreateBotResponse{
		BotID:  botID,
		Status: "created",
		KafkaTopics: models.KafkaTopics{
			Incoming: fmt.Sprintf("bot_%s_incoming", botID),
			Outgoing: fmt.Sprintf("bot_%s_outgoing", botID),
		},
		WebhookURL: webhookURL,
	}

	s.logger.Infow("bot created successfully", "bot_id", botID)
	return response, nil
}

func (s *Service) DeleteBot(ctx context.Context, botID string) error {
	botConfig, err := s.storage.GetBot(ctx, botID)
	if err != nil {
		return fmt.Errorf("bot not found: %w", err)
	}

	s.updateBotStatus(ctx, botID, "deleting")

	s.logger.Infow("deleting telegram webhook", "bot_id", botID)
	if err := s.tgClient.DeleteWebhook(botConfig.BotToken); err != nil {
		s.logger.Errorw("failed to delete webhook", "error", err)
	}

	s.logger.Infow("deleting keda scaledobject", "bot_id", botID)
	if err := s.k8sClient.DeleteScaledObject(ctx, botID); err != nil {
		s.logger.Errorw("failed to delete scaledobject", "error", err)
	}

	s.logger.Infow("deleting kubernetes resources", "bot_id", botID)
	if err := s.k8sClient.DeleteBotResources(ctx, botID); err != nil {
		s.logger.Errorw("failed to delete k8s resources", "error", err)
	}

	s.logger.Infow("deleting kafka topics", "bot_id", botID)
	if err := s.kafkaAdmin.DeleteTopics(ctx, botID); err != nil {
		s.logger.Errorw("failed to delete kafka topics", "error", err)
	}

	if err := s.storage.DeleteBot(ctx, botID, botConfig.BotToken); err != nil {
		return fmt.Errorf("failed to delete bot from storage: %w", err)
	}

	s.logger.Infow("bot deleted successfully", "bot_id", botID)
	return nil
}

// GetBot retrieves bot information
func (s *Service) GetBot(ctx context.Context, botID string) (*models.BotStatusResponse, error) {
	botConfig, err := s.storage.GetBot(ctx, botID)
	if err != nil {
		return nil, err
	}

	// Get current replicas from K8s
	currentReplicas, err := s.k8sClient.GetDeploymentStatus(ctx, botID)
	if err != nil {
		s.logger.Errorw("failed to get deployment status", "error", err)
		currentReplicas = 0
	}

	response := &models.BotStatusResponse{
		BotID:   botConfig.BotID,
		BotName: botConfig.BotName,
		Status:  botConfig.Status,
		Replicas: models.Replicas{
			Current: currentReplicas,
			Min:     botConfig.MinReplicas,
			Max:     botConfig.MaxReplicas,
		},
		KafkaLag:  0, // TODO: Добавить KafkaLag
		CreatedAt: botConfig.CreatedAt,
	}

	return response, nil
}

func (s *Service) UpdateReplicas(ctx context.Context, botID string, req *models.UpdateReplicasRequest) error {
	botConfig, err := s.storage.GetBot(ctx, botID)
	if err != nil {
		return err
	}

	if req.MinReplicas != nil {
		botConfig.MinReplicas = *req.MinReplicas
	}
	if req.MaxReplicas != nil {
		botConfig.MaxReplicas = *req.MaxReplicas
	}

	botConfig.UpdatedAt = time.Now()

	if err := s.storage.SaveBot(ctx, botConfig); err != nil {
		return fmt.Errorf("failed to save bot config: %w", err)
	}

	if err := s.k8sClient.UpdateScaledObject(ctx, botID, req.MinReplicas, req.MaxReplicas); err != nil {
		return fmt.Errorf("failed to update scaledobject: %w", err)
	}

	s.logger.Infow("bot replicas updated", "bot_id", botID)
	return nil
}

func (s *Service) ListBots(ctx context.Context) ([]*models.BotStatusResponse, error) {
	botIDs, err := s.storage.ListBots(ctx)
	if err != nil {
		return nil, err
	}

	bots := make([]*models.BotStatusResponse, 0, len(botIDs))
	for _, botID := range botIDs {
		bot, err := s.GetBot(ctx, botID)
		if err != nil {
			s.logger.Errorw("failed to get bot", "bot_id", botID, "error", err)
			continue
		}
		bots = append(bots, bot)
	}

	return bots, nil
}

func (s *Service) generateBotID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "bot_" + hex.EncodeToString(b)
}

func (s *Service) validateCreateRequest(req *models.CreateBotRequest) error {
	if req.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}
	if req.BotName == "" {
		return fmt.Errorf("bot_name is required")
	}
	if req.WorkerImage == "" {
		return fmt.Errorf("worker_image is required")
	}
	if req.MinReplicas < 0 {
		return fmt.Errorf("min_replicas must be >= 0")
	}
	if req.MaxReplicas < 1 {
		return fmt.Errorf("max_replicas must be >= 1")
	}
	if req.MinReplicas > req.MaxReplicas {
		return fmt.Errorf("min_replicas must be <= max_replicas")
	}
	return nil
}

func (s *Service) updateBotStatus(ctx context.Context, botID, status string) error {
	return s.storage.UpdateBotStatus(ctx, botID, status)
}
