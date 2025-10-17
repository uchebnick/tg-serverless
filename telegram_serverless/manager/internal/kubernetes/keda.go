package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/uchebnick/telegram-serverless/manager/internal/models"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var kedaGVR = schema.GroupVersionResource{
	Group:    "keda.sh",
	Version:  "v1alpha1",
	Resource: "scaledobjects",
}

const scaledObjectTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: telegram-bot
    bot-id: {{ .BotID }}
spec:
  scaleTargetRef:
    name: {{ .DeploymentName }}
  minReplicaCount: {{ .MinReplicas }}
  maxReplicaCount: {{ .MaxReplicas }}
  pollingInterval: 10
  cooldownPeriod: 30
  triggers:
    - type: kafka
      metadata:
        bootstrapServers: {{ .KafkaBrokers }}
        consumerGroup: {{ .ConsumerGroup }}
        topic: {{ .Topic }}
        lagThreshold: "5"
        offsetResetPolicy: earliest
`

type ScaledObjectParams struct {
	Name           string
	Namespace      string
	BotID          string
	DeploymentName string
	MinReplicas    int32
	MaxReplicas    int32
	KafkaBrokers   string
	ConsumerGroup  string
	Topic          string
}

func (c *Client) CreateScaledObject(ctx context.Context, botConfig *models.BotConfig, kafkaBrokers string) error {
	scaledObjectName := fmt.Sprintf("bot-%s-scaler", botConfig.BotID)
	deploymentName := fmt.Sprintf("bot-%s", botConfig.BotID)
	incomingTopic := fmt.Sprintf("bot_%s_incoming", botConfig.BotID)
	consumerGroup := fmt.Sprintf("bot_%s_workers", botConfig.BotID)

	params := ScaledObjectParams{
		Name:           scaledObjectName,
		Namespace:      c.namespace,
		BotID:          botConfig.BotID,
		DeploymentName: deploymentName,
		MinReplicas:    botConfig.MinReplicas,
		MaxReplicas:    botConfig.MaxReplicas,
		KafkaBrokers:   kafkaBrokers,
		ConsumerGroup:  consumerGroup,
		Topic:          incomingTopic,
	}

	tmpl, err := template.New("scaledobject").Parse(scaledObjectTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(buf.Bytes(), obj); err != nil {
		return fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	dynamicClient := c.clientset.RESTClient()
	_, err = dynamicClient.Post().
		AbsPath("/apis/keda.sh/v1alpha1").
		Namespace(c.namespace).
		Resource("scaledobjects").
		Body(buf.Bytes()).
		DoRaw(ctx)

	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create scaledobject: %w", err)
	}

	c.logger.Infow("scaledobject created",
		"scaled_object_name", scaledObjectName,
		"bot_id", botConfig.BotID)

	return nil
}

// DeleteScaledObject deletes a KEDA ScaledObject for a bot
func (c *Client) DeleteScaledObject(ctx context.Context, botID string) error {
	scaledObjectName := fmt.Sprintf("bot-%s-scaler", botID)

	dynamicClient := c.clientset.RESTClient()
	err := dynamicClient.Delete().
		AbsPath("/apis/keda.sh/v1alpha1").
		Namespace(c.namespace).
		Resource("scaledobjects").
		Name(scaledObjectName).
		Do(ctx).
		Error()

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete scaledobject: %w", err)
	}

	c.logger.Infow("scaledobject deleted", "scaled_object_name", scaledObjectName)
	return nil
}

// UpdateScaledObject updates the replica limits of a ScaledObject
func (c *Client) UpdateScaledObject(ctx context.Context, botID string, minReplicas, maxReplicas *int32) error {
	scaledObjectName := fmt.Sprintf("bot-%s-scaler", botID)

	// Get current ScaledObject
	dynamicClient := c.clientset.RESTClient()
	result := dynamicClient.Get().
		AbsPath("/apis/keda.sh/v1alpha1").
		Namespace(c.namespace).
		Resource("scaledobjects").
		Name(scaledObjectName).
		Do(ctx)

	if result.Error() != nil {
		return fmt.Errorf("failed to get scaledobject: %w", result.Error())
	}

	rawBody, err := result.Raw()
	if err != nil {
		return fmt.Errorf("failed to get raw body: %w", err)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(rawBody, obj); err != nil {
		return fmt.Errorf("failed to unmarshal scaledobject: %w", err)
	}

	// Update replica counts
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("spec not found in scaledobject")
	}

	if minReplicas != nil {
		spec["minReplicaCount"] = int64(*minReplicas)
	}
	if maxReplicas != nil {
		spec["maxReplicaCount"] = int64(*maxReplicas)
	}

	if err := unstructured.SetNestedMap(obj.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	// Update ScaledObject
	updatedJSON, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal updated object: %w", err)
	}

	_, err = dynamicClient.Put().
		AbsPath("/apis/keda.sh/v1alpha1").
		Namespace(c.namespace).
		Resource("scaledobjects").
		Name(scaledObjectName).
		Body(updatedJSON).
		DoRaw(ctx)

	if err != nil {
		return fmt.Errorf("failed to update scaledobject: %w", err)
	}

	c.logger.Infow("scaledobject updated", "scaled_object_name", scaledObjectName)
	return nil
}
