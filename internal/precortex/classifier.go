package precortex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// IntentClassifier classifies user messages into intent categories using LLM
type IntentClassifier struct {
	logger         *zap.Logger
	aiServicesURL  string
	client         *http.Client
	requestCount   atomic.Int64
	cacheHits      atomic.Int64
}

// NewIntentClassifier creates a new LLM-based intent classifier
func NewIntentClassifier(logger *zap.Logger) *IntentClassifier {
	// Use Docker service name for container-to-container communication
	// Check if running in Docker by testing if rmk-ai-services is reachable
	aiURL := "http://rmk-ai-services:8000"
	return &IntentClassifier{
		logger:        logger,
		aiServicesURL: aiURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetAIServicesURL sets the AI services URL
func (ic *IntentClassifier) SetAIServicesURL(url string) {
	ic.aiServicesURL = url
	ic.logger.Info("IntentClassifier: AI services URL updated", zap.String("url", url))
}

// Classify determines the intent of a user message using LLM
func (ic *IntentClassifier) Classify(message string) Intent {
	ic.requestCount.Add(1)

	// Normalize message
	msg := strings.TrimSpace(message)
	if len(msg) < 2 {
		return IntentGreeting
	}

	// Fast path for obvious greetings (no LLM call needed)
	lowerMsg := strings.ToLower(msg)
	for _, greeting := range []string{"hi", "hello", "hey", "yo", "sup", "thanks", "thank you", "thx", "bye", "goodbye"} {
		if lowerMsg == greeting || lowerMsg == greeting+"!" || lowerMsg == greeting+"." {
			ic.cacheHits.Add(1)
			return IntentGreeting
		}
	}

	// Call LLM for classification
	intent, err := ic.classifyWithLLM(msg)
	if err != nil {
		ic.logger.Warn("LLM classification failed, using fallback", zap.Error(err))
		// Fallback: use simple heuristic
		return ic.fallbackClassify(msg)
	}

	ic.logger.Debug("LLM classified intent", zap.String("intent", string(intent)))
	return intent
}

// classifyWithLLM calls the AI service to classify intent
func (ic *IntentClassifier) classifyWithLLM(message string) (Intent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reqBody := map[string]string{
		"query": message,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		ic.aiServicesURL+"/classify-intent",
		bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ic.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("classify-intent returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Intent string `json:"intent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Map string to Intent
	switch result.Intent {
	case "GREETING":
		return IntentGreeting, nil
	case "NAVIGATION":
		return IntentNavigation, nil
	case "FACT_RETRIEVAL":
		return IntentFactRetrieval, nil
	default:
		return IntentComplex, nil
	}
}

// fallbackClassify provides simple rule-based fallback when LLM is unavailable
func (ic *IntentClassifier) fallbackClassify(message string) Intent {
	lowerMsg := strings.ToLower(message)

	// Check for navigation keywords
	for _, kw := range []string{"go to", "open", "show my", "navigate to", "settings", "dashboard", "profile"} {
		if strings.Contains(lowerMsg, kw) {
			return IntentNavigation
		}
	}

	// Check for fact retrieval patterns
	if strings.Contains(lowerMsg, "?") ||
		strings.HasPrefix(lowerMsg, "what") ||
		strings.HasPrefix(lowerMsg, "where") ||
		strings.HasPrefix(lowerMsg, "who") ||
		strings.HasPrefix(lowerMsg, "when") ||
		strings.HasPrefix(lowerMsg, "how") ||
		strings.HasPrefix(lowerMsg, "do i") ||
		strings.HasPrefix(lowerMsg, "did i") ||
		strings.Contains(lowerMsg, "my name") ||
		strings.Contains(lowerMsg, "my email") ||
		strings.Contains(lowerMsg, "i live") {
		return IntentFactRetrieval
	}

	// Default to complex
	return IntentComplex
}

// Stats returns classification statistics
func (ic *IntentClassifier) Stats() (total int64, cacheHits int64) {
	return ic.requestCount.Load(), ic.cacheHits.Load()
}
