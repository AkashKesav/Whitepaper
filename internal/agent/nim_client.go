// Package agent provides NVIDIA NIM (NVIDIA Inference Microservice) client for LLM inference
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultNIMBaseURL is the default base URL for NVIDIA NIM API
	DefaultNIMBaseURL = "https://integrate.api.nvidia.com/v1"
	// DefaultNIMModel is the default model to use
	DefaultNIMModel = "meta/llama-3.1-405b-instruct"
	// DefaultNIMEmbedModel is the default embedding model
	DefaultNIMEmbedModel = "nvidia/nv-embedqa-mistral-7b"
)

// NIMClient provides access to NVIDIA NIM API for LLM inference and embeddings
type NIMClient struct {
	baseURL    string
	apiKey     string
	model      string
	embedModel string
	httpClient *http.Client
	logger     *zap.Logger
}

// NIMConfig holds configuration for the NIM client
type NIMConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	EmbedModel string
	Timeout    time.Duration
}

// NewNIMClient creates a new NVIDIA NIM client
func NewNIMClient(config NIMConfig, logger *zap.Logger) *NIMClient {
	if config.BaseURL == "" {
		config.BaseURL = DefaultNIMBaseURL
	}
	if config.Model == "" {
		config.Model = DefaultNIMModel
	}
	if config.EmbedModel == "" {
		config.EmbedModel = DefaultNIMEmbedModel
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &NIMClient{
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		model:      config.Model,
		embedModel: config.EmbedModel,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
	}
}

// ChatMessage represents a message in the chat completion request
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NIMChatRequest represents a chat completion request to NIM
type NIMChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	MaxTokens int          `json:"max_tokens,omitempty"`
	Temperature float64    `json:"temperature,omitempty"`
	TopP     float64       `json:"top_p,omitempty"`
}

// NIMChatResponse represents a chat completion response from NIM
type NIMChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []NIMChoice `json:"choices"`
	Usage   NIMUsage    `json:"usage"`
}

// NIMChoice represents a choice in the chat response
type NIMChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// NIMUsage represents token usage information
type NIMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NIMEmbedRequest represents an embedding request to NIM
type NIMEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// NIMEmbedResponse represents an embedding response from NIM
type NIMEmbedResponse struct {
	Object string        `json:"object"`
	Data   []NIMEmbedData `json:"data"`
	Model  string        `json:"model"`
	Usage  NIMUsage      `json:"usage"`
}

// NIMEmbedData represents embedding data for a single input
type NIMEmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Chat sends a chat completion request to NIM
func (c *NIMClient) Chat(ctx context.Context, messages []ChatMessage) (*NIMChatResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("NIM API key not configured")
	}

	reqBody := NIMChatRequest{
		Model:    c.model,
		Messages: messages,
		MaxTokens: 4096,
		Temperature: 0.7,
		TopP: 0.9,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.Debug("Sending NIM chat request",
		zap.String("model", c.model),
		zap.Int("messages", len(messages)))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NIM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp NIMChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("NIM chat response received",
		zap.Int("prompt_tokens", chatResp.Usage.PromptTokens),
		zap.Int("completion_tokens", chatResp.Usage.CompletionTokens))

	return &chatResp, nil
}

// Embed sends an embedding request to NIM
func (c *NIMClient) Embed(ctx context.Context, texts []string) (*NIMEmbedResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("NIM API key not configured")
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided for embedding")
	}

	reqBody := NIMEmbedRequest{
		Model: c.embedModel,
		Input: texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.Debug("Sending NIM embed request",
		zap.String("model", c.embedModel),
		zap.Int("texts", len(texts)))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NIM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embedResp NIMEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("NIM embed response received",
		zap.Int("embeddings", len(embedResp.Data)))

	return &embedResp, nil
}

// SetAPIKey updates the API key for the NIM client
func (c *NIMClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// IsConfigured returns true if the NIM client is properly configured with an API key
func (c *NIMClient) IsConfigured() bool {
	return c.apiKey != ""
}
