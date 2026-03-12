package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/yourusername/kam/config"
)

type claudeProvider struct {
	apiKey string
	model  string
	maxTok int
}

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClaude(cfg *config.Config) (Provider, error) {
	if cfg.AI.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY env var'ı veya config'de api_key gerekli")
	}
	return &claudeProvider{
		apiKey: cfg.AI.APIKey,
		model:  cfg.AI.Model,
		maxTok: cfg.AI.MaxTokens,
	}, nil
}

func (c *claudeProvider) Name() string {
	return fmt.Sprintf("Claude (%s)", c.model)
}

func (c *claudeProvider) Complete(ctx context.Context, system, prompt string) (string, error) {
	reqBody := claudeRequest{
		Model:     c.model,
		MaxTokens: c.maxTok,
		System:    system,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("yanıt parse edilemedi: %w", err)
	}

	if claudeResp.Error != nil {
		return "", fmt.Errorf("Claude API hatası: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("boş yanıt alındı")
	}

	return claudeResp.Content[0].Text, nil
}
