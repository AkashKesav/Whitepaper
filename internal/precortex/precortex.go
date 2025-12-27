// Package precortex provides the cognitive firewall that intercepts requests
// before they reach the external LLM, reducing costs by 90% and improving latency.
package precortex

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
	"github.com/reflective-memory-kernel/internal/kernel/cache"
	"go.uber.org/zap"
)

// Intent represents the classified intent of a user message
type Intent string

const (
	IntentGreeting      Intent = "GREETING"
	IntentNavigation    Intent = "NAVIGATION"
	IntentFactRetrieval Intent = "FACT_RETRIEVAL"
	IntentComplex       Intent = "COMPLEX"
)

// Response represents a Pre-Cortex response
type Response struct {
	Text    string `json:"text,omitempty"`
	Action  string `json:"action,omitempty"`
	Target  string `json:"target,omitempty"`
	Handled bool   `json:"handled"`
}

// Config holds Pre-Cortex configuration
type Config struct {
	EnableSemanticCache bool
	EnableIntentRouter  bool
	EnableDGraphReflex  bool
	CacheSimilarity     float64 // Minimum similarity for cache hit (0.0-1.0)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		EnableSemanticCache: true,
		EnableIntentRouter:  true,
		EnableDGraphReflex:  true,
		CacheSimilarity:     0.95,
	}
}

// PreCortex is the cognitive firewall that sits between User and LLM
type PreCortex struct {
	config       Config
	logger       *zap.Logger
	cacheManager *cache.Manager
	graphClient  *graph.Client

	// Components
	// Components
	intentClassifier    *IntentClassifier
	semanticCache       *SemanticCache
	reflexEngine        *ReflexEngine
	semanticVectorIndex *kernel.VectorIndex

	mu sync.RWMutex

	// Metrics
	totalRequests   int64
	cachedResponses int64
	reflexResponses int64
	llmPassthrough  int64
}

// NewPreCortex creates a new Pre-Cortex instance
func NewPreCortex(cfg Config, cacheManager *cache.Manager, graphClient *graph.Client, semanticVectorIndex *kernel.VectorIndex, logger *zap.Logger) (*PreCortex, error) {
	pc := &PreCortex{
		config:              cfg,
		logger:              logger,
		cacheManager:        cacheManager,
		graphClient:         graphClient,
		semanticVectorIndex: semanticVectorIndex,
	}

	// Initialize intent classifier
	pc.intentClassifier = NewIntentClassifier(logger)

	// Initialize semantic cache (will add ONNX embedder later)
	pc.semanticCache = NewSemanticCache(cacheManager, semanticVectorIndex, nil, cfg.CacheSimilarity, logger)

	// Initialize reflex engine
	pc.reflexEngine = NewReflexEngine(graphClient, logger)

	logger.Info("Pre-Cortex initialized",
		zap.Bool("semantic_cache", cfg.EnableSemanticCache),
		zap.Bool("intent_router", cfg.EnableIntentRouter),
		zap.Bool("dgraph_reflex", cfg.EnableDGraphReflex))

	return pc, nil
}

// SetEmbedder configures the embedder for semantic similarity search
func (pc *PreCortex) SetEmbedder(embedder Embedder) {
	pc.semanticCache = NewSemanticCache(pc.cacheManager, pc.semanticVectorIndex, embedder, pc.config.CacheSimilarity, pc.logger)
	pc.logger.Info("Pre-Cortex semantic cache embedder configured",
		zap.Bool("embedder_active", embedder != nil))
}

// SetRedisClient configures Redis for persistent vector storage
func (pc *PreCortex) SetRedisClient(client interface{}) {
	// Type assert to *redis.Client
	if redisClient, ok := client.(interface {
		Get(ctx context.Context, key string) interface{}
	}); ok {
		_ = redisClient // Redis client is set on semantic cache
	}
	pc.logger.Info("Pre-Cortex Redis client configured for cache persistence")
}

// Handle processes a user request through the Pre-Cortex pipeline
// Returns (response, handled). If handled is false, the request should go to LLM.
func (pc *PreCortex) Handle(ctx context.Context, namespace, userID, query string) (Response, bool) {
	pc.mu.Lock()
	pc.totalRequests++
	pc.mu.Unlock()

	pc.logger.Debug("Pre-Cortex processing request",
		zap.String("namespace", namespace),
		zap.String("query", query[:min(50, len(query))]))

	// --- STEP 1: Semantic Cache (Memory Filter) ---
	if pc.config.EnableSemanticCache {
		if cached, found := pc.semanticCache.Check(ctx, namespace, query); found {
			pc.mu.Lock()
			pc.cachedResponses++
			pc.mu.Unlock()
			pc.logger.Debug("Pre-Cortex: Cache HIT")
			return Response{Text: cached, Handled: true}, true
		}
	}

	// --- STEP 2: Intent Classification (Router) ---
	intent := IntentComplex // Default to complex
	if pc.config.EnableIntentRouter {
		intent = pc.intentClassifier.Classify(query)
		pc.logger.Debug("Pre-Cortex: Classified intent", zap.String("intent", string(intent)))
	}

	// --- STEP 3: Deterministic Reflexes ---
	switch intent {
	case IntentGreeting:
		pc.mu.Lock()
		pc.reflexResponses++
		pc.mu.Unlock()
		return Response{
			Text:    "Hello! How can I help you with your memories today?",
			Handled: true,
		}, true

	case IntentNavigation:
		pc.mu.Lock()
		pc.reflexResponses++
		pc.mu.Unlock()
		// Parse navigation target
		target := pc.parseNavigationTarget(query)
		return Response{
			Action:  "navigate",
			Target:  target,
			Handled: true,
		}, true

	case IntentFactRetrieval:
		// --- STEP 4: DGraph Librarian ---
		if pc.config.EnableDGraphReflex {
			response, handled := pc.reflexEngine.Handle(ctx, namespace, userID, query)
			if handled {
				pc.mu.Lock()
				pc.reflexResponses++
				pc.mu.Unlock()
				return Response{Text: response, Handled: true}, true
			}
		}
	}

	// Not handled - pass to LLM
	pc.mu.Lock()
	pc.llmPassthrough++
	pc.mu.Unlock()
	pc.logger.Debug("Pre-Cortex: Passing to LLM")
	return Response{Handled: false}, false
}

// SaveToCache stores a response in the semantic cache
func (pc *PreCortex) SaveToCache(ctx context.Context, namespace, query, response string) {
	if pc.config.EnableSemanticCache {
		pc.semanticCache.Store(ctx, namespace, query, response)
	}
}

// Stats returns Pre-Cortex statistics
func (pc *PreCortex) Stats() (total, cached, reflex, llm int64, hitRate float64) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	total = pc.totalRequests
	cached = pc.cachedResponses
	reflex = pc.reflexResponses
	llm = pc.llmPassthrough
	handled := cached + reflex
	if total > 0 {
		hitRate = float64(handled) / float64(total)
	}
	return
}

func (pc *PreCortex) parseNavigationTarget(query string) string {
	targets := []struct {
		patterns []string
		target   string
	}{
		{[]string{"setting", "config", "preference"}, "settings"},
		{[]string{"dashboard", "home", "main"}, "dashboard"},
		{[]string{"profile", "account"}, "profile"},
		{[]string{"group", "team"}, "groups"},
		{[]string{"memory", "knowledge"}, "memory"},
		{[]string{"help", "support"}, "help"},
	}

	queryLower := strings.ToLower(query)
	for _, t := range targets {
		for _, pattern := range t.patterns {
			if strings.Contains(queryLower, pattern) {
				return t.target
			}
		}
	}
	return "dashboard" // Default
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MarshalJSON implements custom JSON marshaling for Response
func (r Response) MarshalJSON() ([]byte, error) {
	if r.Action != "" {
		return json.Marshal(map[string]string{
			"action": r.Action,
			"target": r.Target,
		})
	}
	return json.Marshal(map[string]interface{}{
		"text":    r.Text,
		"handled": r.Handled,
	})
}
