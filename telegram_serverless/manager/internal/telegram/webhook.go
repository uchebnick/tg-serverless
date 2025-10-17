package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

func NewClient(apiURL string, logger *zap.SugaredLogger) *Client {
	return &Client{
		baseURL: apiURL,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) SetWebhook(botToken, webhookURL string, certificate []byte) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", botToken)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("url", webhookURL); err != nil {
		return fmt.Errorf("failed to write url field: %w", err)
	}

	if len(certificate) > 0 {
		part, err := writer.CreateFormFile("certificate", "ca.crt")
		if err != nil {
			return fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := part.Write(certificate); err != nil {
			return fmt.Errorf("failed to write certificate data: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result struct {
			Description string `json:"description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("failed to set webhook: %s", result.Description)
	}

	return nil
}

// DeleteWebhook deletes the webhook for a bot
func (c *Client) DeleteWebhook(botToken string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", botToken)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !result.Ok {
		return fmt.Errorf("telegram handlers error: %s", result.Description)
	}

	c.logger.Info("webhook deleted successfully")
	return nil
}
