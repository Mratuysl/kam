package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Mratuysl/kam/config"
)

// ─── OpenAI ───────────────────────────────────────────────────────────────────

type openaiProvider struct {
	apiKey string
	model  string
	maxTok int
}

type openaiRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewOpenAI(cfg *config.Config) (Provider, error) {
	if cfg.AI.APIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY env var'ı veya config'de api_key gerekli")
	}
	model := cfg.AI.Model
	if model == "" {
		model = "gpt-4o"
	}
	return &openaiProvider{apiKey: cfg.AI.APIKey, model: model, maxTok: cfg.AI.MaxTokens}, nil
}

func (o *openaiProvider) Name() string { return fmt.Sprintf("OpenAI (%s)", o.model) }

func (o *openaiProvider) Complete(ctx context.Context, system, prompt string) (string, error) {
	reqBody := openaiRequest{
		Model:     o.model,
		MaxTokens: o.maxTok,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result openaiResponse
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return "", fmt.Errorf("OpenAI API hatası: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("boş yanıt")
	}
	return result.Choices[0].Message.Content, nil
}

// ─── Ollama (Local) ───────────────────────────────────────────────────────────

type ollamaProvider struct {
	baseURL string
	model   string
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

func NewOllama(cfg *config.Config) (Provider, error) {
	url := cfg.AI.OllamaURL
	if url == "" {
		url = "http://localhost:11434"
	}
	model := cfg.AI.Model
	if model == "" {
		model = "llama3"
	}
	return &ollamaProvider{baseURL: url, model: model}, nil
}

func (o *ollamaProvider) Name() string { return fmt.Sprintf("Ollama (%s)", o.model) }

func (o *ollamaProvider) Complete(ctx context.Context, system, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		System: system,
		Stream: false,
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST",
		o.baseURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama'ya bağlanılamadı (%s): %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result ollamaResponse
	json.Unmarshal(body, &result)

	if result.Error != "" {
		return "", fmt.Errorf("Ollama hatası: %s", result.Error)
	}
	return result.Response, nil
}
// ─── Replicate ────────────────────────────────────────────────────────────────

type replicateProvider struct {
	apiKey string
	model  string
}

type replicateRequest struct {
	Input replicateInput `json:"input"`
}

type replicateInput struct {
	System string `json:"system"`
	Prompt string `json:"prompt"`
}

type replicatePrediction struct {
	ID     string      `json:"id"`
	Status interface{} `json:"status"`
	Output interface{} `json:"output"`
	Error  interface{} `json:"error"`
}

func NewReplicate(cfg *config.Config) (Provider, error) {
	if cfg.AI.APIKey == "" {
		return nil, fmt.Errorf("REPLICATE_API_TOKEN gerekli")
	}
	model := cfg.AI.Model
	if model == "" {
		model = "anthropic/claude-haiku-4-5"
	}
	return &replicateProvider{apiKey: cfg.AI.APIKey, model: model}, nil
}

func (r *replicateProvider) Name() string {
	return fmt.Sprintf("Replicate (%s)", r.model)
}

func (r *replicateProvider) Complete(ctx context.Context, system, prompt string) (string, error) {
	// Replicate system prompt'u bazen yok sayıyor, direkt prompt'a göm
	fullPrompt := system + "\n\n---\nKULLANICI İSTEĞİ: " + prompt

	reqBody := replicateRequest{
		Input: replicateInput{
			System: system,
			Prompt: fullPrompt,
		},
	}

	data, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("https://api.replicate.com/v1/models/%s/predictions", r.model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Prefer", "wait")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Replicate API hatası: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var prediction replicatePrediction
	if err := json.Unmarshal(body, &prediction); err != nil {
		return "", fmt.Errorf("yanıt parse edilemedi: %w", err)
	}

	if prediction.Error != nil && prediction.Error != "" {
        return "", fmt.Errorf("Replicate hatası: %v", prediction.Error)
    }

	switch v := prediction.Output.(type) {
	case string:
		return v, nil
	case []interface{}:
		result := ""
		for _, item := range v {
			if s, ok := item.(string); ok {
				result += s
			}
		}
		return result, nil
	}

	return "", fmt.Errorf("beklenmeyen output formatı")
}
