// Package router provides multi-provider LLM routing for the RMK system
// Supports GLM (Zhipu AI), NVIDIA NIM, OpenAI, Anthropic, and local Ollama
package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reflective-memory-kernel/internal/jsonx"
	"go.uber.org/zap"
)

// Provider represents an LLM provider
type Provider string

const (
	ProviderGLM       Provider = "glm"
	ProviderNVIDIA    Provider = "nvidia"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOllama    Provider = "ollama"
	ProviderMiniMax   Provider = "minimax"
)

// Config holds the router configuration
type Config struct {
	GLMKey       string
	NVIDIAKey    string
	OpenAIKey    string
	AnthropicKey string
	MiniMaxKey   string
	OllamaURL    string

	// Default provider to use
	DefaultProvider Provider

	// Request timeouts
	RequestTimeout  time.Duration
	ConnectTimeout  time.Duration
}

// DefaultConfig returns default configuration from environment variables
func DefaultConfig() *Config {
	cfg := &Config{
		GLMKey:       strings.TrimSpace(os.Getenv("GLM_API_KEY")),
		NVIDIAKey:    strings.TrimSpace(os.Getenv("NVIDIA_API_KEY")),
		OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		MiniMaxKey:   os.Getenv("MINIMAX_API_KEY"),
		OllamaURL:    getEnvOrDefault("OLLAMA_URL", "http://localhost:11434"),
		RequestTimeout: 180 * time.Second,
		ConnectTimeout: 30 * time.Second,
	}

	// Determine default provider
	if cfg.GLMKey != "" {
		cfg.DefaultProvider = ProviderGLM
	} else if cfg.NVIDIAKey != "" {
		cfg.DefaultProvider = ProviderNVIDIA
	} else {
		cfg.DefaultProvider = ProviderOllama
	}

	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// Router handles LLM request routing to multiple providers
type Router struct {
	config  *Config
	client  *http.Client
	logger  *zap.Logger
	mu      sync.RWMutex

	// Runtime state
	providers      map[Provider]bool
	defaultProvider Provider
}

// New creates a new LLM router
func New(cfg *Config, logger *zap.Logger) *Router {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	r := &Router{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:         logger,
		providers:      make(map[Provider]bool),
		defaultProvider: cfg.DefaultProvider,
	}

	// Determine available providers
	if cfg.GLMKey != "" {
		r.providers[ProviderGLM] = true
	}
	if cfg.NVIDIAKey != "" {
		r.providers[ProviderNVIDIA] = true
	}
	if cfg.OpenAIKey != "" {
		r.providers[ProviderOpenAI] = true
	}
	if cfg.AnthropicKey != "" {
		r.providers[ProviderAnthropic] = true
	}
	if cfg.MiniMaxKey != "" {
		r.providers[ProviderMiniMax] = true
	}
	// Ollama is always available as local fallback
	r.providers[ProviderOllama] = true

	return r
}

// GenerateRequest represents a generation request
type GenerateRequest struct {
	Query           string            `json:"query"`
	Context         string            `json:"context,omitempty"`
	Alerts          []string          `json:"alerts,omitempty"`
	Provider        Provider          `json:"provider,omitempty"`
	Model           string            `json:"model,omitempty"`
	Format          string            `json:"format,omitempty"`
	SystemInstruction string          `json:"system_instruction,omitempty"`
	UserAPIKeys     map[string]string `json:"user_api_keys,omitempty"`
}

// GenerateResponse represents a generation response
type GenerateResponse struct {
	Content    string `json:"content"`
	Provider   Provider `json:"provider"`
	Model      string    `json:"model"`
	TokensUsed int       `json:"tokens_used,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// Generate sends a generation request to the appropriate LLM provider
func (r *Router) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	start := time.Now()

	// Use default provider if none specified
	provider := req.Provider
	if provider == "" {
		provider = r.defaultProvider
	}

	// Build system prompt
	system := req.SystemInstruction
	if system == "" {
		system = r.buildSystemPrompt(req.Context, req.Alerts)
	}

	// Route to appropriate provider
	var content string
	var err error

	switch provider {
	case ProviderGLM:
		apiKey := r.getAPIKey(req.UserAPIKeys, "glm", r.config.GLMKey)
		model := req.Model
		if model == "" {
			model = "glm-4.5"
		}
		content, err = r.callGLM(ctx, system, req.Query, model, apiKey)

	case ProviderNVIDIA:
		apiKey := r.getAPIKey(req.UserAPIKeys, "nim", r.config.NVIDIAKey)
		model := req.Model
		if model == "" {
			model = "meta/llama-3.1-70b-instruct"
		}
		content, err = r.callNVIDIA(ctx, system, req.Query, model, apiKey)

	case ProviderOpenAI:
		apiKey := r.getAPIKey(req.UserAPIKeys, "openai", r.config.OpenAIKey)
		model := req.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
		content, err = r.callOpenAI(ctx, system, req.Query, model, apiKey)

	case ProviderAnthropic:
		apiKey := r.getAPIKey(req.UserAPIKeys, "anthropic", r.config.AnthropicKey)
		model := req.Model
		if model == "" {
			model = "claude-3-haiku-20240307"
		}
		content, err = r.callAnthropic(ctx, system, req.Query, model, apiKey)

	case ProviderOllama:
		model := req.Model
		if model == "" {
			model = "llama3.2"
		}
		content, err = r.callOllama(ctx, system, req.Query, model)

	default:
		// Try fallback
		if r.providers[ProviderGLM] {
			content, err = r.callGLM(ctx, system, req.Query, "glm-4-plus", r.config.GLMKey)
			provider = ProviderGLM
		} else {
			content, err = r.callOllama(ctx, system, req.Query, "llama3.2")
			provider = ProviderOllama
		}
	}

	if err != nil {
		return nil, fmt.Errorf("provider %s failed: %w", provider, err)
	}

	// Strip thinking tags if present
	content = stripThinkingTags(content)

	return &GenerateResponse{
		Content:  content,
		Provider: provider,
		Model:    req.Model,
		Duration: time.Since(start),
	}, nil
}

// VisionRequest represents a vision generation request
type VisionRequest struct {
	ImageBase64 string   `json:"image_base64"`
	Prompt      string   `json:"prompt"`
	Model       string   `json:"model,omitempty"`
}

// GenerateVision sends a vision request to the appropriate provider
func (r *Router) GenerateVision(ctx context.Context, req *VisionRequest) (string, error) {
	model := req.Model
	if model == "" {
		model = "minimaxai/minimax-m2"
	}

	// Try NVIDIA first if available
	if r.config.NVIDIAKey != "" {
		content, err := r.callNVIDIAVision(ctx, req.Prompt, req.ImageBase64, model)
		if err == nil {
			return content, nil
		}
		r.logger.Warn("NVIDIA vision failed, trying fallback", zap.Error(err))
	}

	// Fallback to MiniMax
	if r.config.MiniMaxKey != "" {
		return r.callMiniMaxVision(ctx, req.Prompt, req.ImageBase64)
	}

	return "", fmt.Errorf("no vision provider configured")
}

// ExtractJSON generates and parses JSON output
func (r *Router) ExtractJSON(ctx context.Context, prompt string, provider Provider, model string) (map[string]interface{}, error) {
	system := "You are a precise entity extraction engine. Output JSON only."

	req := &GenerateRequest{
		Query:           prompt,
		Provider:        provider,
		Model:           model,
		SystemInstruction: system,
	}

	resp, err := r.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return parseJSONFromResponse(resp.Content)
}

// buildSystemPrompt builds the system prompt with context and alerts
func (r *Router) buildSystemPrompt(context string, alerts []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are a helpful AI assistant with access to the user's personal memory database. ")
	prompt.WriteString("When answering questions, you MUST check the MEMORY CONTEXT section below first. ")
	prompt.WriteString("If the answer is in the MEMORY CONTEXT, use it to answer directly.")

	if context != "" && strings.TrimSpace(context) != "" && !strings.Contains(context, "No relevant memories") {
		prompt.WriteString("\n\n### MEMORY CONTEXT (ANSWER FROM THIS!):\n")
		prompt.WriteString(context)
		prompt.WriteString("\n### END MEMORY CONTEXT\n\n")
		prompt.WriteString("IMPORTANT: The information above is from the user's memory. Use it to answer their question!")
	} else {
		prompt.WriteString("\n\n### MEMORY CONTEXT:\n(No memories found)\n###\n\n")
		prompt.WriteString("Say: 'I don't have that stored yet. Would you like to tell me?'")
	}

	if len(alerts) > 0 {
		prompt.WriteString("\n\nAlerts:\n")
		for _, alert := range alerts {
			prompt.WriteString("- ")
			prompt.WriteString(alert)
			prompt.WriteString("\n")
		}
	}

	return prompt.String()
}

// getAPIKey gets the API key from user keys or default
func (r *Router) getAPIKey(userKeys map[string]string, keyName string, defaultKey string) string {
	if userKeys != nil {
		if key, ok := userKeys[keyName]; ok && key != "" {
			return key
		}
	}
	return defaultKey
}

// callGLM calls the GLM (Zhipu AI) API
func (r *Router) callGLM(ctx context.Context, system, query, model, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no GLM API key available")
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": query},
		},
		"max_tokens": 1000,
	}

	return r.makeRequest(ctx, "https://open.bigmodel.cn/api/paas/v4/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	})
}

// callNVIDIA calls the NVIDIA NIM API
func (r *Router) callNVIDIA(ctx context.Context, system, query, model, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no NVIDIA API key available")
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": query},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
	}

	return r.makeRequest(ctx, "https://integrate.api.nvidia.com/v1/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	})
}

// callNVIDIAVision calls the NVIDIA NIM Vision API
func (r *Router) callNVIDIAVision(ctx context.Context, prompt, imageBase64, model string) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:image/jpeg;base64," + imageBase64,
					}},
				},
			},
		},
		"max_tokens":  2048,
		"temperature": 0.3,
	}

	return r.makeRequest(ctx, "https://integrate.api.nvidia.com/v1/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + r.config.NVIDIAKey,
		"Content-Type":  "application/json",
	})
}

// callMiniMaxVision calls the MiniMax Vision API
func (r *Router) callMiniMaxVision(ctx context.Context, prompt, imageBase64 string) (string, error) {
	reqBody := map[string]interface{}{
		"model": "abab6.5-chat",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:image/jpeg;base64," + imageBase64,
					}},
				},
			},
		},
	}

	return r.makeRequest(ctx, "https://api.minimax.chat/v1/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + r.config.MiniMaxKey,
		"Content-Type":  "application/json",
	})
}

// callOpenAI calls the OpenAI API
func (r *Router) callOpenAI(ctx context.Context, system, query, model, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no OpenAI API key available")
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": query},
		},
		"max_tokens": 1000,
	}

	return r.makeRequest(ctx, "https://api.openai.com/v1/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	})
}

// callAnthropic calls the Anthropic API
func (r *Router) callAnthropic(ctx context.Context, system, query, model, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no Anthropic API key available")
	}

	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 1000,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": query},
		},
	}

	return r.makeRequest(ctx, "https://api.anthropic.com/v1/messages", reqBody, map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	})
}

// callOllama calls the local Ollama API
func (r *Router) callOllama(ctx context.Context, system, query, model string) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": query},
		},
		"stream": false,
	}

	url := fmt.Sprintf("%s/api/chat", r.config.OllamaURL)
	return r.makeRequest(ctx, url, reqBody, map[string]string{
		"Content-Type": "application/json",
	})
}

// makeRequest makes an HTTP request to an LLM API
func (r *Router) makeRequest(ctx context.Context, url string, body map[string]interface{}, headers map[string]string) (string, error) {
	jsonBody, err := jsonx.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := jsonx.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content from response
	return extractContent(result)
}

// extractContent extracts the content from an LLM API response
func extractContent(result map[string]interface{}) (string, error) {
	// Try OpenAI/NIM/GLM format
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	// Try Anthropic format
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if block, ok := content[0].(map[string]interface{}); ok {
			if text, ok := block["text"].(string); ok {
				return text, nil
			}
		}
	}

	// Try Ollama format
	if message, ok := result["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok {
			return content, nil
		}
	}

	// Try direct content field
	if content, ok := result["content"].(string); ok {
		return content, nil
	}

	return "", fmt.Errorf("could not extract content from response")
}

// stripThinkingTags removes thinking tags from AI responses
func stripThinkingTags(content string) string {
	// Pattern to match <think>...</think> tags
	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	return strings.TrimSpace(re.ReplaceAllString(content, ""))
}

// parseJSONFromResponse parses JSON from an LLM response
func parseJSONFromResponse(response string) (map[string]interface{}, error) {
	if response == "" {
		return make(map[string]interface{}), nil
	}

	// Find first '[' or '{'
	startIdx := -1
	for i, c := range response {
		if c == '[' || c == '{' {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return make(map[string]interface{}), nil
	}

	textToParse := response[startIdx:]

	// Try to find valid JSON by trying different end points
	closer := byte('}') // Default to object closer
	if response[startIdx] == '[' {
		closer = byte(']')
	}

	// Find all occurrences of the closer and try them
	var lastValid interface{}
	for i := len(textToParse) - 1; i >= 0; i-- {
		if textToParse[i] == closer {
			candidate := textToParse[:i+1]
			var result interface{}
			if err := jsonx.Unmarshal([]byte(candidate), &result); err == nil {
				// Successfully parsed
				switch v := result.(type) {
				case map[string]interface{}:
					return v, nil
				case []interface{}:
					return map[string]interface{}{"items": v}, nil
				default:
					lastValid = v
				}
			}
		}
	}

	if lastValid != nil {
		switch v := lastValid.(type) {
		case map[string]interface{}:
			return v, nil
		case []interface{}:
			return map[string]interface{}{"items": v}, nil
		}
	}

	return make(map[string]interface{}), nil
}

// GetProviders returns the list of available providers
func (r *Router) GetProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []Provider
	for p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetDefaultProvider returns the default provider
func (r *Router) GetDefaultProvider() Provider {
	return r.defaultProvider
}

// SetDefaultProvider sets the default provider
func (r *Router) SetDefaultProvider(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.providers[provider] {
		return fmt.Errorf("provider %s is not available", provider)
	}

	r.defaultProvider = provider
	return nil
}

// IsProviderAvailable checks if a provider is available
func (r *Router) IsProviderAvailable(provider Provider) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[provider]
}

// GetStats returns router statistics
func (r *Router) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]string, 0, len(r.providers))
	for p := range r.providers {
		providers = append(providers, string(p))
	}

	return map[string]interface{}{
		"default_provider": string(r.defaultProvider),
		"available_providers": providers,
		"total_providers":  len(providers),
	}
}

// EncodeBase64 encodes a byte slice to base64
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes a base64 string
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// ParseInt parses a string to int
func ParseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// ParseFloat parses a string to float64
func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ParseBool parses a string to bool
func ParseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

// String returns the string representation of a provider
func (p Provider) String() string {
	return string(p)
}

// IsValidProvider checks if a provider is valid
func IsValidProvider(provider string) bool {
	switch Provider(provider) {
	case ProviderGLM, ProviderNVIDIA, ProviderOpenAI, ProviderAnthropic, ProviderOllama, ProviderMiniMax:
		return true
	default:
		return false
	}
}

// ParseProvider parses a provider string
func ParseProvider(s string) (Provider, error) {
	p := Provider(s)
	if !IsValidProvider(s) {
		return "", fmt.Errorf("invalid provider: %s", s)
	}
	return p, nil
}

// ProviderInfo holds information about a provider
type ProviderInfo struct {
	Name         Provider `json:"name"`
	DisplayName  string   `json:"display_name"`
	Available    bool     `json:"available"`
	RequiresAuth bool     `json:"requires_auth"`
	Models       []string `json:"models,omitempty"`
}

// GetProviderInfo returns information about all providers
func (r *Router) GetProviderInfo() []ProviderInfo {
	return []ProviderInfo{
		{
			Name:         ProviderGLM,
			DisplayName:  "GLM (Zhipu AI)",
			Available:    r.providers[ProviderGLM],
			RequiresAuth: true,
			Models:       []string{"glm-4.5", "glm-4-plus", "glm-4-flash"},
		},
		{
			Name:         ProviderNVIDIA,
			DisplayName:  "NVIDIA NIM",
			Available:    r.providers[ProviderNVIDIA],
			RequiresAuth: true,
			Models:       []string{"meta/llama-3.1-70b-instruct", "minimaxai/minimax-m2"},
		},
		{
			Name:         ProviderOpenAI,
			DisplayName:  "OpenAI",
			Available:    r.providers[ProviderOpenAI],
			RequiresAuth: true,
			Models:       []string{"gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo"},
		},
		{
			Name:         ProviderAnthropic,
			DisplayName:  "Anthropic",
			Available:    r.providers[ProviderAnthropic],
			RequiresAuth: true,
			Models:       []string{"claude-3-haiku-20240307", "claude-3-sonnet", "claude-3-opus"},
		},
		{
			Name:         ProviderMiniMax,
			DisplayName:  "MiniMax",
			Available:    r.providers[ProviderMiniMax],
			RequiresAuth: true,
			Models:       []string{"abab6.5-chat", "abab6.5s-chat"},
		},
		{
			Name:         ProviderOllama,
			DisplayName:  "Ollama (Local)",
			Available:    true,
			RequiresAuth: false,
			Models:       []string{"llama3.2", "llama3.1", "mistral"},
		},
	}
}
