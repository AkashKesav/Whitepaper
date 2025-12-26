// Package memory provides the Cold Path batching for the Memory Kernel.
// This batches messages and summarizes them before writing to DGraph.
package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// BatchSize is the number of messages to accumulate before summarizing
	BatchSize = 20
	// BatchInterval is the maximum time to wait before processing a batch
	BatchInterval = 2 * time.Minute
)

// Message represents a message in the batch
type Message struct {
	UserID    string    `json:"user_id"`
	Query     string    `json:"query"`
	Response  string    `json:"response"`
	Timestamp time.Time `json:"timestamp"`
}

// Batcher collects messages and periodically summarizes them
type Batcher struct {
	aiServicesURL string
	messages      map[string][]Message // userID -> messages
	mu            sync.Mutex
	logger        *zap.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	onSummary     func(userID, summary string, messages []Message) error
}

// NewBatcher creates a new message batcher
func NewBatcher(aiServicesURL string, logger *zap.Logger, onSummary func(string, string, []Message) error) *Batcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Batcher{
		aiServicesURL: aiServicesURL,
		messages:      make(map[string][]Message),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		onSummary:     onSummary,
	}
}

// Start begins the background batching loop
func (b *Batcher) Start() {
	go b.batchLoop()
	b.logger.Info("Cold path batcher started", zap.Duration("interval", BatchInterval))
}

// Stop stops the batcher
func (b *Batcher) Stop() {
	b.cancel()
}

// Add adds a message to the batch for the given user
func (b *Batcher) Add(userID, query, response string) {
	// Skip chitchat messages
	if isChitchat(query) {
		b.logger.Debug("Skipping chitchat message", zap.String("query", query[:min(30, len(query))]))
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	msg := Message{
		UserID:    userID,
		Query:     query,
		Response:  response,
		Timestamp: time.Now(),
	}

	b.messages[userID] = append(b.messages[userID], msg)

	// Trigger immediate processing if batch is full
	if len(b.messages[userID]) >= BatchSize {
		go b.processBatch(userID)
	}
}

// batchLoop periodically processes all batches
func (b *Batcher) batchLoop() {
	ticker := time.NewTicker(BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.processAllBatches()
		}
	}
}

// processAllBatches processes batches for all users
func (b *Batcher) processAllBatches() {
	b.mu.Lock()
	userIDs := make([]string, 0, len(b.messages))
	for userID := range b.messages {
		if len(b.messages[userID]) > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	b.mu.Unlock()

	for _, userID := range userIDs {
		b.processBatch(userID)
	}
}

// processBatch summarizes and processes a user's batch
func (b *Batcher) processBatch(userID string) {
	b.mu.Lock()
	messages := b.messages[userID]
	if len(messages) == 0 {
		b.mu.Unlock()
		return
	}
	// Clear the batch
	b.messages[userID] = nil
	b.mu.Unlock()

	b.logger.Info("Processing message batch",
		zap.String("user_id", userID),
		zap.Int("message_count", len(messages)))

	// Create summary using LLM
	summary, err := b.summarizeMessages(messages)
	if err != nil {
		b.logger.Error("Failed to summarize messages", zap.Error(err))
		// Put messages back on failure
		b.mu.Lock()
		b.messages[userID] = append(b.messages[userID], messages...)
		b.mu.Unlock()
		return
	}

	// Callback with summary
	if b.onSummary != nil {
		if err := b.onSummary(userID, summary, messages); err != nil {
			b.logger.Error("Summary callback failed", zap.Error(err))
		}
	}

	b.logger.Info("Batch summarized successfully",
		zap.String("user_id", userID),
		zap.Int("messages", len(messages)),
		zap.Int("summary_len", len(summary)))
}

// summarizeMessages calls the AI service to summarize messages
func (b *Batcher) summarizeMessages(messages []Message) (string, error) {
	// Build prompt
	var sb strings.Builder
	sb.WriteString("Summarize these conversation turns into key facts about the user. Extract only important, persistent information:\n\n")
	for i, msg := range messages {
		sb.WriteString(fmt.Sprintf("%d. User: %s\n   AI: %s\n\n", i+1, msg.Query, msg.Response))
	}

	type SummarizeRequest struct {
		Query   string `json:"query"`
		Context string `json:"context"`
	}

	reqBody := SummarizeRequest{
		Query:   "Summarize the key facts from these conversations.",
		Context: sb.String(),
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", b.aiServicesURL+"/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI service returned %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}

// isChitchat detects low-value messages that shouldn't be stored
func isChitchat(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))

	// Very short messages are usually chitchat
	if len(query) < 5 {
		return true
	}

	// Common chitchat patterns
	chitchatPatterns := []string{
		"hello", "hi", "hey", "ok", "okay", "thanks", "thank you",
		"bye", "goodbye", "cool", "nice", "great", "yes", "no",
		"sure", "alright", "good", "fine", "hmm", "hm", "oh",
	}

	for _, pattern := range chitchatPatterns {
		if query == pattern {
			return true
		}
	}

	return false
}
