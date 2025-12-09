// Package agent implements the Front-End Agent - "The Consciousness".
// This is the lightweight, user-facing conversational agent optimized for low-latency interaction.
package agent

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

// Config holds configuration for the Front-End Agent
type Config struct {
	NATSAddress     string
	MemoryKernelURL string
	AIServicesURL   string
	ResponseTimeout time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		NATSAddress:     "nats://localhost:4222",
		MemoryKernelURL: "http://localhost:9000",
		AIServicesURL:   "http://localhost:8000",
		ResponseTimeout: 10 * time.Second,
	}
}

// Agent is the Front-End Agent - fast, conversational interface
type Agent struct {
	config   Config
	logger   *zap.Logger
	natsConn *nats.Conn
	js       nats.JetStreamContext
	mkClient *MKClient
	aiClient *AIClient

	// Active conversations
	conversations map[string]*Conversation
	convMu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// Conversation tracks an active conversation session
type Conversation struct {
	ID        string
	UserID    string
	StartedAt time.Time
	Turns     []Turn
	mu        sync.Mutex
}

// Turn represents one conversational turn
type Turn struct {
	Timestamp time.Time
	UserQuery string
	Response  string
	Latency   time.Duration
}

// New creates a new Front-End Agent
func New(cfg Config, logger *zap.Logger) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:        cfg,
		logger:        logger,
		conversations: make(map[string]*Conversation),
		ctx:           ctx,
		cancel:        cancel,
	}

	return agent, nil
}

// Start initializes the agent
func (a *Agent) Start() error {
	a.logger.Info("Starting Front-End Agent...")

	// Connect to NATS
	natsConn, err := nats.Connect(a.config.NATSAddress,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
	)
	if err != nil {
		return err
	}
	a.natsConn = natsConn

	js, err := natsConn.JetStream()
	if err != nil {
		return err
	}
	a.js = js

	// Initialize clients
	a.mkClient = NewMKClient(a.config.MemoryKernelURL, a.logger)
	a.aiClient = NewAIClient(a.config.AIServicesURL, a.logger)

	a.logger.Info("Front-End Agent started successfully")
	return nil
}

// Stop gracefully shuts down the agent
func (a *Agent) Stop() error {
	a.cancel()
	if a.natsConn != nil {
		a.natsConn.Close()
	}
	a.logger.Info("Front-End Agent stopped")
	return nil
}

// Chat handles a user message and returns a response
func (a *Agent) Chat(ctx context.Context, userID, conversationID, message string) (string, error) {
	startTime := time.Now()

	a.logger.Debug("Processing chat message",
		zap.String("user_id", userID),
		zap.String("message", message))

	// Get or create conversation
	conv := a.getOrCreateConversation(userID, conversationID)

	// Step 1: Consult Memory Kernel for context (async-aware)
	consultReq := &graph.ConsultationRequest{
		UserID:          userID,
		Query:           message,
		MaxResults:      5,
		IncludeInsights: true,
	}

	var mkResponse *graph.ConsultationResponse
	var mkErr error

	// Non-blocking MK consultation with timeout
	mkDone := make(chan struct{})
	go func() {
		mkResponse, mkErr = a.mkClient.Consult(ctx, consultReq)
		close(mkDone)
	}()

	// Wait for MK with timeout (don't block conversation)
	select {
	case <-mkDone:
		if mkErr != nil {
			a.logger.Warn("MK consultation failed, proceeding without context", zap.Error(mkErr))
		}
	case <-time.After(2 * time.Second):
		a.logger.Warn("MK consultation timed out, proceeding without context")
		mkErr = context.DeadlineExceeded
	}

	// Step 2: Generate response using AI
	var contextBrief string
	var proactiveAlerts []string
	if mkResponse != nil && mkErr == nil {
		contextBrief = mkResponse.SynthesizedBrief
		proactiveAlerts = mkResponse.ProactiveAlerts
		a.logger.Info("Context brief from MK",
			zap.String("brief", contextBrief),
			zap.Int("facts_count", len(mkResponse.RelevantFacts)))
	}

	response, err := a.aiClient.GenerateResponse(ctx, message, contextBrief, proactiveAlerts)
	if err != nil {
		return "", err
	}

	latency := time.Since(startTime)

	// Step 3: Record this turn
	conv.mu.Lock()
	conv.Turns = append(conv.Turns, Turn{
		Timestamp: time.Now(),
		UserQuery: message,
		Response:  response,
		Latency:   latency,
	})
	conv.mu.Unlock()

	// Step 4: Stream transcript to Memory Kernel (async, non-blocking)
	go a.streamTranscript(userID, conversationID, message, response)

	a.logger.Info("Chat response generated",
		zap.Duration("latency", latency),
		zap.Bool("had_context", mkResponse != nil))

	return response, nil
}

// streamTranscript asynchronously sends the conversation to the Memory Kernel
func (a *Agent) streamTranscript(userID, conversationID, userQuery, response string) {
	event := &graph.TranscriptEvent{
		UserID:         userID,
		ConversationID: conversationID,
		Timestamp:      time.Now(),
		UserQuery:      userQuery,
		AIResponse:     response,
	}

	if err := kernel.PublishTranscript(a.js, event); err != nil {
		a.logger.Warn("Failed to stream transcript", zap.Error(err))
	}
}

// getOrCreateConversation gets or creates a conversation
func (a *Agent) getOrCreateConversation(userID, conversationID string) *Conversation {
	a.convMu.Lock()
	defer a.convMu.Unlock()

	if conv, ok := a.conversations[conversationID]; ok {
		return conv
	}

	conv := &Conversation{
		ID:        conversationID,
		UserID:    userID,
		StartedAt: time.Now(),
		Turns:     make([]Turn, 0),
	}
	a.conversations[conversationID] = conv
	return conv
}

// GetStats returns agent statistics
func (a *Agent) GetStats() map[string]interface{} {
	a.convMu.RLock()
	defer a.convMu.RUnlock()

	totalTurns := 0
	var totalLatency time.Duration
	for _, conv := range a.conversations {
		conv.mu.Lock()
		totalTurns += len(conv.Turns)
		for _, turn := range conv.Turns {
			totalLatency += turn.Latency
		}
		conv.mu.Unlock()
	}

	avgLatency := time.Duration(0)
	if totalTurns > 0 {
		avgLatency = totalLatency / time.Duration(totalTurns)
	}

	return map[string]interface{}{
		"active_conversations": len(a.conversations),
		"total_turns":          totalTurns,
		"average_latency_ms":   avgLatency.Milliseconds(),
	}
}

// MarshalJSON for Turn
func (t Turn) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp string `json:"timestamp"`
		UserQuery string `json:"user_query"`
		Response  string `json:"response"`
		LatencyMs int64  `json:"latency_ms"`
	}{
		Timestamp: t.Timestamp.Format(time.RFC3339),
		UserQuery: t.UserQuery,
		Response:  t.Response,
		LatencyMs: t.Latency.Milliseconds(),
	})
}
