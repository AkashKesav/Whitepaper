<<<<<<< HEAD
// Package agent implements the Front-End Agent - "The Consciousness".
// This is the lightweight, user-facing conversational agent optimized for low-latency interaction.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/precortex"
)

// Config holds configuration for the Front-End Agent
type Config struct {
	NATSAddress     string
	MemoryKernelURL string
	AIServicesURL   string
	RedisAddress    string
	ResponseTimeout time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		NATSAddress:     "nats://localhost:4222",
		MemoryKernelURL: "http://127.0.0.1:9000",
		AIServicesURL:   "http://localhost:8000",
		RedisAddress:    "127.0.0.1:6379",
		ResponseTimeout: 10 * time.Second,
	}
}

// Agent is the Front-End Agent - fast, conversational interface
type Agent struct {
	config      Config
	logger      *zap.Logger
	natsConn    *nats.Conn
	js          nats.JetStreamContext
	mkClient    *MKClient
	aiClient    *AIClient
	RedisClient *redis.Client        // Exposed for user authentication
	preCortex   *precortex.PreCortex // Cognitive firewall for cost reduction

	// Active conversations
	conversations map[string]*Conversation
	convMu        sync.RWMutex

	// Direct Ingestion (Zero-Copy)
	ingestChan chan *graph.TranscriptEvent

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

	// Initialize Redis for user authentication
	a.RedisClient = redis.NewClient(&redis.Options{
		Addr: a.config.RedisAddress,
	})
	if err := a.RedisClient.Ping(a.ctx).Err(); err != nil {
		a.logger.Warn("Failed to connect to Redis for auth, user credentials will not persist", zap.Error(err))
		// Don't fail startup - just continue without auth persistence
	}

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

// SetIngestChannel enables direct ingestion mode (Zero-Copy)
func (a *Agent) SetIngestChannel(ch chan *graph.TranscriptEvent) {
	a.ingestChan = ch
	a.logger.Info("Direct Ingestion Channel configured (Zero-Copy enabled)")
}

// SetKernel enables direct consultation mode (Zero-Copy)
func (a *Agent) SetKernel(k MemoryKernel) {
	if a.mkClient != nil {
		a.mkClient.SetDirectKernel(k)
		a.logger.Info("Direct Kernel access configured (Zero-Copy enabled)")
	}
}

// SetPreCortex configures the Pre-Cortex cognitive firewall for cost reduction
func (a *Agent) SetPreCortex(pc *precortex.PreCortex) {
	a.preCortex = pc
	a.logger.Info("Pre-Cortex cognitive firewall configured (90% cost reduction enabled)")
}

// Chat handles a user message and returns a response
func (a *Agent) Chat(ctx context.Context, userID, conversationID, namespace, message string) (string, error) {
	startTime := time.Now()

	a.logger.Debug("Processing chat message",
		zap.String("user_id", userID),
		zap.String("namespace", namespace),
		zap.String("message", message))

	// --- PRE-CORTEX: Try to handle locally first (90% cost reduction) ---
	if a.preCortex != nil {
		pcResponse, handled := a.preCortex.Handle(ctx, namespace, userID, message)
		if handled {
			latency := time.Since(startTime)
			a.logger.Info("Pre-Cortex handled request",
				zap.Duration("latency", latency),
				zap.Bool("handled", true))

			// Record turn and stream transcript
			conv := a.getOrCreateConversation(userID, conversationID)
			conv.mu.Lock()
			conv.Turns = append(conv.Turns, Turn{
				Timestamp: time.Now(),
				UserQuery: message,
				Response:  pcResponse.Text,
				Latency:   latency,
			})
			conv.mu.Unlock()

			// Stream transcript (still learn from Pre-Cortex interactions)
			go a.streamTranscript(userID, conversationID, namespace, message, pcResponse.Text)

			return pcResponse.Text, nil
		}
	}

	// --- CONTINUE TO LLM (Pre-Cortex did not handle) ---

	// Get or create conversation
	conv := a.getOrCreateConversation(userID, conversationID)

	// Step 1: Consult Memory Kernel for context (async-aware)
	consultReq := &graph.ConsultationRequest{
		UserID:          userID,
		Namespace:       namespace, // Pass namespace to MK
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
		return "", fmt.Errorf("failed to generate response: %w", err)
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
	go a.streamTranscript(userID, conversationID, namespace, message, response)

	a.logger.Info("Chat response generated",
		zap.Duration("latency", latency),
		zap.Bool("had_context", mkResponse != nil))

	// Step 5: Save to Pre-Cortex semantic cache for future reuse
	if a.preCortex != nil {
		a.preCortex.SaveToCache(ctx, namespace, message, response)
	}

	return response, nil
}

// Speculate triggers a speculative lookup (fire and forget usually)
func (a *Agent) Speculate(ctx context.Context, userID, namespace, partialMessage string) {
	if len(partialMessage) < 5 {
		return
	}

	// Create consultation request
	req := &graph.ConsultationRequest{
		UserID:    userID,
		Namespace: namespace,
		Query:     partialMessage,
	}

	// Async call to Kernel
	go func() {
		if err := a.mkClient.Speculate(context.Background(), req); err != nil {
			a.logger.Debug("Speculation failed", zap.Error(err))
		}
	}()
}

// streamTranscript asynchronously sends the conversation to the Memory Kernel
func (a *Agent) streamTranscript(userID, conversationID, namespace, userQuery, aiResponse string) {
	event := graph.TranscriptEvent{
		ID:             uuid.New().String(),
		UserID:         userID,
		Namespace:      namespace, // Pass namespace to Ingestion
		ConversationID: conversationID,
		Timestamp:      time.Now(),
		UserQuery:      userQuery,
		AIResponse:     aiResponse,
	}
	// ... rest of function (unchanged usually)
	// Zero-Copy Path: Send directly to Kernel via channel if configured
	if a.ingestChan != nil {
		select {
		case a.ingestChan <- &event:
			a.logger.Debug("Transcript sent via direct channel")
			return
		default:
			a.logger.Warn("Ingest channel full, falling back to NATS/Dropping")
		}
	}

	// Legacy Path: NATS
	if a.natsConn == nil {
		a.logger.Warn("NATS connection is nil, and no direct channel configured. Transcript lost.")
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		a.logger.Error("Failed to marshal transcript event", zap.Error(err))
		return
	}

	subject := fmt.Sprintf("transcripts.%s", userID)
	if err := a.natsConn.Publish(subject, data); err != nil {
		a.logger.Error("Failed to publish transcript event", zap.Error(err))
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
=======
// Package agent implements the Front-End Agent - "The Consciousness".
// This is the lightweight, user-facing conversational agent optimized for low-latency interaction.
package agent

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

// Config holds configuration for the Front-End Agent
type Config struct {
	NATSAddress     string
	MemoryKernelURL string
	AIServicesURL   string
	RedisAddress    string
	ResponseTimeout time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		NATSAddress:     "nats://localhost:4222",
		MemoryKernelURL: "http://127.0.0.1:9000",
		AIServicesURL:   "http://localhost:8000",
		RedisAddress:    "127.0.0.1:6379",
		ResponseTimeout: 10 * time.Second,
	}
}

// Agent is the Front-End Agent - fast, conversational interface
type Agent struct {
	config      Config
	logger      *zap.Logger
	natsConn    *nats.Conn
	js          nats.JetStreamContext
	mkClient    MemoryKernelClient
	aiClient    *AIClient
	RedisClient *redis.Client // Exposed for user authentication

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

// SetMemoryClient sets the Memory Kernel client (used for dependency injection of local client)
func (a *Agent) SetMemoryClient(client MemoryKernelClient) {
	a.mkClient = client
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

	// Ensure the TRANSCRIPTS stream exists (may be created by Memory Kernel too)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "TRANSCRIPTS",
		Subjects: []string{"transcripts.*"},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour * 30, // 30 days retention
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		a.logger.Warn("Failed to create NATS stream", zap.Error(err))
		// Don't fail - stream might be created by Memory Kernel
	} else {
		a.logger.Info("NATS TRANSCRIPTS stream ready")
	}

	// Initialize clients
	if a.mkClient == nil {
		a.mkClient = NewMKClient(a.config.MemoryKernelURL, a.logger)
	}
	a.aiClient = NewAIClient(a.config.AIServicesURL, a.logger)

	// Initialize Redis for user authentication
	a.RedisClient = redis.NewClient(&redis.Options{
		Addr: a.config.RedisAddress,
	})
	if err := a.RedisClient.Ping(a.ctx).Err(); err != nil {
		a.logger.Warn("Failed to connect to Redis for auth, user credentials will not persist", zap.Error(err))
		// Don't fail startup - just continue without auth persistence
	}

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

	// Step 4: Store in hot cache for instant retrieval (async, non-blocking)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.mkClient.StoreInHotCache(ctx, userID, message, response, conversationID)
	}()

	// Step 5: Stream transcript to Memory Kernel (async, non-blocking)
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

// ConsultMemory performs a lightweight memory consultation for prefetching.
// Used for speculative execution when user is typing.
func (a *Agent) ConsultMemory(ctx context.Context, userID, query string) string {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	consultReq := &graph.ConsultationRequest{
		UserID:     userID,
		Query:      query,
		MaxResults: 3, // Fewer results for prefetch
	}

	resp, err := a.mkClient.Consult(ctx, consultReq)
	if err != nil {
		a.logger.Debug("Prefetch consultation failed", zap.Error(err))
		return ""
	}

	if resp != nil {
		return resp.SynthesizedBrief
	}
	return ""
}

>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
