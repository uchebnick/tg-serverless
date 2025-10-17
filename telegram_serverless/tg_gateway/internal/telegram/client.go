package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

var apiResp struct {
	Ok          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

func NewClient(baseURL string, logger *zap.SugaredLogger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) CallMethod(botToken, method string, params map[string]interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL, botToken, method)

	var reqBody []byte
	var err error
	if params != nil {
		reqBody, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("telegram handlers error",
			zap.String("method", method),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)))
		return nil, fmt.Errorf("telegram handlers returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !apiResp.Ok {
		return nil, fmt.Errorf("telegram handlers error [%d]: %s", apiResp.ErrorCode, apiResp.Description)
	}

	return apiResp.Result, nil
}
