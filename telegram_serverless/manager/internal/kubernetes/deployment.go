package kubernetes

import (
	"context"
	"fmt"

	"github.com/uchebnick/telegram-serverless/manager/internal/models"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (c *Client) CreateBotResources(ctx context.Context, botConfig *models.BotConfig, kafkaBrokers string) error {
	if err := c.createSecret(ctx, botConfig); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	if err := c.createDeployment(ctx, botConfig, kafkaBrokers); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	c.logger.Infow("bot resources created",
		"bot_id", botConfig.BotID,
		"deployment_name", fmt.Sprintf("bot-%s", botConfig.BotID))

	return nil
}

func (c *Client) createSecret(ctx context.Context, botConfig *models.BotConfig) error {
	secretName := fmt.Sprintf("bot-%s-secrets", botConfig.BotID)

	// Prepare secret data
	secretData := make(map[string][]byte)
	secretData["BOT_TOKEN"] = []byte(botConfig.BotToken)

	// Add custom env vars
	for key, value := range botConfig.EnvVars {
		secretData[key] = []byte(value)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app":    "telegram-bot",
				"bot-id": botConfig.BotID,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	_, err := c.clientset.CoreV1().Secrets(c.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	c.logger.Infow("secret created", "secret_name", secretName)
	return nil
}

func (c *Client) createDeployment(ctx context.Context, botConfig *models.BotConfig, kafkaBrokers string) error {
	deploymentName := fmt.Sprintf("bot-%s", botConfig.BotID)
	secretName := fmt.Sprintf("bot-%s-secrets", botConfig.BotID)

	incomingTopic := fmt.Sprintf("bot_%s_incoming", botConfig.BotID)
	outgoingTopic := fmt.Sprintf("bot_%s_outgoing", botConfig.BotID)

	replicas := int32(0)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app":    "telegram-bot",
				"bot-id": botConfig.BotID,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":    "telegram-bot",
					"bot-id": botConfig.BotID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "telegram-bot",
						"bot-id": botConfig.BotID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "bot",
							Image: botConfig.WorkerImage,
							Env: []corev1.EnvVar{
								{
									Name:  "BOT_ID",
									Value: botConfig.BotID,
								},
								{
									Name:  "BOT_TOKEN",
									Value: botConfig.BotToken,
								},
								{
									Name:  "SIDECAR_URL",
									Value: "http://localhost:8081",
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{},
						},
						{
							Name:  "sidecar",
							Image: c.sidecarImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8081,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "metrics",
									ContainerPort: 9091,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PORT",
									Value: "8081",
								},
								{
									Name:  "METRICS_PORT",
									Value: "9091",
								},
								{
									Name:  "LOG_LEVEL",
									Value: "info",
								},
								{
									Name:  "INCOMING_TOPIC",
									Value: incomingTopic,
								},
								{
									Name:  "OUTGOING_TOPIC",
									Value: outgoingTopic,
								},
								{
									Name:  "KAFKA_CONSUMER_GROUP",
									Value: fmt.Sprintf("bot_%s_workers", botConfig.BotID),
								},
								{
									Name:  "BOT_TOKEN",
									Value: botConfig.BotToken,
								},
								{
									Name:  "KAFKA_BROKERS",
									Value: kafkaBrokers,
								},
								{
									Name:  "KAFKA_CONSUMER_GROUP",
									Value: fmt.Sprintf("bot_%s_workers", botConfig.BotID),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 3,
								PeriodSeconds:       5,
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	_, err := c.clientset.AppsV1().Deployments(c.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	c.logger.Infow("deployment created", "deployment_name", deploymentName)
	return nil
}

func (c *Client) DeleteBotResources(ctx context.Context, botID string) error {
	deploymentName := fmt.Sprintf("bot-%s", botID)
	secretName := fmt.Sprintf("bot-%s-secrets", botID)

	deletePolicy := metav1.DeletePropagationForeground
	if err := c.clientset.AppsV1().Deployments(c.namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	if err := c.clientset.CoreV1().Secrets(c.namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	c.logger.Infow("bot resources deleted", "bot_id", botID)
	return nil
}

// GetDeploymentStatus gets the current status of a bot deployment
func (c *Client) GetDeploymentStatus(ctx context.Context, botID string) (int32, error) {
	deploymentName := fmt.Sprintf("bot-%s", botID)

	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	return deployment.Status.ReadyReplicas, nil
}
