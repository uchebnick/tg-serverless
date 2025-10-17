package models

import "time"

type BotConfig struct {
	BotID       string            `json:"bot_id"`
	BotToken    string            `json:"bot_token"`
	BotName     string            `json:"bot_name"`
	WorkerImage string            `json:"worker_image"`
	MinReplicas int32             `json:"min_replicas"`
	MaxReplicas int32             `json:"max_replicas"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Status      string            `json:"status"` // created, running, failed, deleting
}

type CreateBotRequest struct {
	BotToken    string            `json:"bot_token"`
	BotName     string            `json:"bot_name"`
	WorkerImage string            `json:"worker_image"`
	MinReplicas int32             `json:"min_replicas"`
	MaxReplicas int32             `json:"max_replicas"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
}

type CreateBotResponse struct {
	BotID       string      `json:"bot_id"`
	Status      string      `json:"status"`
	KafkaTopics KafkaTopics `json:"kafka_topics"`
	WebhookURL  string      `json:"webhook_url"`
}

type KafkaTopics struct {
	Incoming string `json:"incoming"`
	Outgoing string `json:"outgoing"`
}

type UpdateReplicasRequest struct {
	MinReplicas *int32 `json:"min_replicas,omitempty"`
	MaxReplicas *int32 `json:"max_replicas,omitempty"`
}

type BotStatusResponse struct {
	BotID     string    `json:"bot_id"`
	BotName   string    `json:"bot_name"`
	Status    string    `json:"status"`
	Replicas  Replicas  `json:"replicas"`
	KafkaLag  int64     `json:"kafka_lag"`
	CreatedAt time.Time `json:"created_at"`
}

type Replicas struct {
	Current int32 `json:"current"`
	Min     int32 `json:"min"`
	Max     int32 `json:"max"`
}
