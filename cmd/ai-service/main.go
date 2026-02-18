// AI Services - gnet-based server for SLM orchestration
// Provides extraction, curation, synthesis, and generation endpoints.
// This is the Go port of ai/main.py
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/ai/curation"
	"github.com/reflective-memory-kernel/internal/ai/router"
	"github.com/reflective-memory-kernel/internal/ai/synthesis"
	"github.com/reflective-memory-kernel/internal/ingester"
	"github.com/reflective-memory-kernel/internal/server"
	"github.com/reflective-memory-kernel/internal/validation"
	"github.com/reflective-memory-kernel/internal/vectorindex"
	"go.uber.org/zap"
)

// AIService holds all the AI services
type AIService struct {
	llmRouter   *router.Router
	curation    *curation.Service
	synthesis   *synthesis.Service
	ingester    *ingester.Service
	vectorIndex *vectorindex.IndexBuilder
	logger      *zap.Logger
}

// Config holds the server configuration
type Config struct {
	Host         string
	Port         int
	NVIDIAKey    string
	GLMKey       string
	OpenAIKey    string
	AnthropicKey string
	OllamaURL    string
}

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg := loadConfig()

	// Initialize router
	routerConfig := &router.Config{
		NVIDIAKey:     cfg.NVIDIAKey,
		GLMKey:        cfg.GLMKey,
		OpenAIKey:     cfg.OpenAIKey,
		AnthropicKey:  cfg.AnthropicKey,
		OllamaURL:     cfg.OllamaURL,
		// Don't set DefaultProvider - let router auto-detect based on available keys
		// Priority: GLM > NVIDIA > OpenAI > Anthropic > Ollama
	}

	llmRouter := router.New(routerConfig, logger)

	// Initialize AI services
	aiSvc := &AIService{
		llmRouter:   llmRouter,
		curation:    curation.New(llmRouter, logger),
		synthesis:   synthesis.New(llmRouter, logger),
		ingester:    ingester.New(nil, llmRouter, logger),
		vectorIndex: vectorindex.NewIndexBuilder(10, 1536, logger),
		logger:      logger,
	}

	// Create gnet engine
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	opts := &server.Options{
		Network:   "tcp",
		Multicore: true,
		Logger:    logger,
	}
	engine := server.New(addr, opts)

	// Setup routes
	setupRoutes(engine, aiSvc)

	logger.Info("AI Services server starting",
		zap.String("address", addr),
		zap.Bool("nvidia_key", cfg.NVIDIAKey != ""),
		zap.Bool("glm_key", cfg.GLMKey != ""),
	)

	// Start server
	if err := engine.Start(); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}

func loadConfig() *Config {
	return &Config{
		Host:         getEnv("AI_SERVICE_HOST", "0.0.0.0"),
		Port:         getEnvInt("AI_SERVICE_PORT", 8001),
		NVIDIAKey:    getEnv("NVIDIA_API_KEY", ""),
		GLMKey:       getEnv("GLM_API_KEY", ""),
		OpenAIKey:    getEnv("OPENAI_API_KEY", ""),
		AnthropicKey: getEnv("ANTHROPIC_API_KEY", ""),
		OllamaURL:    getEnv("OLLAMA_URL", "http://localhost:11434"),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		var intVal int
		if _, err := fmt.Sscanf(val, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func setupRoutes(engine *server.Engine, svc *AIService) {
	// Health check
	engine.GET("/health", func(req *server.Request) *server.Response {
		return server.JSON(map[string]string{"status": "healthy", "service": "ai-service"}, 200)
	})

	// Debug router
	engine.GET("/debug-router", func(req *server.Request) *server.Response {
		return server.JSON(map[string]any{
			"service": "ai-service",
			"status":  "running",
		}, 200)
	})

	// Entity extraction
	engine.POST("/extract", func(req *server.Request) *server.Response {
		var r ExtractRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.extractEntities(req, r)
	})

	// Fact curation
	engine.POST("/curate", func(req *server.Request) *server.Response {
		var r CurationRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.curateFacts(req, r)
	})

	// Synthesis
	engine.POST("/synthesize", func(req *server.Request) *server.Response {
		var r SynthesisRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.synthesizeBrief(req, r)
	})

	// Insight synthesis
	engine.POST("/synthesize-insight", func(req *server.Request) *server.Response {
		var r InsightRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.synthesizeInsight(req, r)
	})

	// Generate response
	engine.POST("/generate", func(req *server.Request) *server.Response {
		var r GenerateRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.generateResponse(req, r)
	})

	// Expand query
	engine.POST("/expand-query", func(req *server.Request) *server.Response {
		var r ExpandQueryRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.expandQuery(req, r)
	})

	// Vision extraction
	engine.POST("/extract-vision", func(req *server.Request) *server.Response {
		var r VisionExtractRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.extractVision(req, r)
	})

	// Document ingestion
	engine.POST("/ingest", func(req *server.Request) *server.Response {
		var r IngestRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.ingestDocument(req, r)
	})

	// Entity resolution
	engine.POST("/resolve-entity", func(req *server.Request) *server.Response {
		var r ResolveEntityRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.resolveEntity(req, r)
	})

	// Classify intent
	engine.POST("/classify-intent", func(req *server.Request) *server.Response {
		var r map[string]any
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.classifyIntent(req, r)
	})

	// Semantic search
	engine.POST("/semantic-search", func(req *server.Request) *server.Response {
		var r SemanticSearchRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.semanticSearch(req, r)
	})

	// Cognify batch (for migration)
	engine.POST("/cognify-batch", func(req *server.Request) *server.Response {
		var r CognifyBatchRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.cognifyBatch(req, r)
	})

	// Summarize batch (for wisdom layer entity extraction)
	engine.POST("/summarize_batch", func(req *server.Request) *server.Response {
		var r SummarizeBatchRequest
		if err := server.ParseJSON(req, &r); err != nil {
			return server.JSON(map[string]string{"error": "invalid request", "details": err.Error()}, 400)
		}
		return svc.summarizeBatch(req, r)
	})
}

// Request/Response types

type ExtractRequest struct {
	UserQuery  string `json:"user_query"`
	AIResponse string `json:"ai_response"`
	Context    string `json:"context,omitempty"`
}

type ExtractedEntity struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Relations   []interface{}          `json:"relations,omitempty"`
	Confidence  float64                `json:"confidence,omitempty"`
	Source      string                 `json:"source,omitempty"`
}

type CurationRequest struct {
	Node1Name        string `json:"node1_name"`
	Node1Description string `json:"node1_description"`
	Node1CreatedAt   string `json:"node1_created_at"`
	Node2Name        string `json:"node2_name"`
	Node2Description string `json:"node2_description"`
	Node2CreatedAt   string `json:"node2_created_at"`
}

type CurationResponse struct {
	WinnerIndex int    `json:"winner_index"` // 1 or 2
	Reason      string `json:"reason"`
}

type SynthesisRequest struct {
	Query           string             `json:"query"`
	Context         string             `json:"context,omitempty"`
	Facts           []synthesis.Fact   `json:"facts,omitempty"`
	Insights        []synthesis.Insight `json:"insights,omitempty"`
	ProactiveAlerts []string           `json:"proactive_alerts,omitempty"`
}

type SynthesisResponse struct {
	Brief      string  `json:"brief"`
	Confidence float64 `json:"confidence"`
}

type InsightRequest struct {
	Node1Name        string `json:"node1_name"`
	Node1Type        string `json:"node1_type"`
	Node1Description string `json:"node1_description,omitempty"`
	Node2Name        string `json:"node2_name"`
	Node2Type        string `json:"node2_type"`
	Node2Description string `json:"node2_description,omitempty"`
	PathExists       bool   `json:"path_exists"`
	PathLength       int    `json:"path_length"`
}

type InsightResponse struct {
	HasInsight       bool    `json:"has_insight"`
	InsightType      string  `json:"insight_type,omitempty"`
	Summary          string  `json:"summary,omitempty"`
	ActionSuggestion string  `json:"action_suggestion,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
}

type GenerateRequest struct {
	Query           string            `json:"query"`
	Context         string            `json:"context,omitempty"`
	ProactiveAlerts []string          `json:"proactive_alerts,omitempty"`
	UserAPIKeys     map[string]string `json:"user_api_keys,omitempty"` // Per-user API keys
}

type GenerateResponse struct {
	Response string `json:"response"`
}

type ExpandQueryRequest struct {
	Query string `json:"query"`
}

type ExpandQueryResponse struct {
	OriginalQuery string   `json:"original_query"`
	SearchTerms   []string `json:"search_terms"`
	EntityNames   []string `json:"entity_names"`
}

type VisionExtractRequest struct {
	ImageBase64 string `json:"image_base64"`
	Prompt      string `json:"prompt,omitempty"`
}

type VisionExtractResponse struct {
	RawResponse   string                 `json:"raw_response"`
	Entities      []map[string]interface{} `json:"entities,omitempty"`
	Relationships []map[string]interface{} `json:"relationships,omitempty"`
	Insights      []string               `json:"insights,omitempty"`
}

type IngestRequest struct {
	Text          string `json:"text,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
	DocumentType  string `json:"document_type,omitempty"`
	Filename      string `json:"filename,omitempty"`
}

type IngestResponse struct {
	Entities      []map[string]interface{} `json:"entities"`
	Relationships []map[string]interface{} `json:"relationships,omitempty"`
	Chunks        []map[string]interface{} `json:"chunks,omitempty"`
	Stats         map[string]interface{}   `json:"stats,omitempty"`
	Summary       string                   `json:"summary,omitempty"`
	VectorTree    map[string]*vectorindex.VectorNode `json:"vector_tree,omitempty"`
}

type ResolveEntityRequest struct {
	Entity     string   `json:"entity"`
	Candidates []string `json:"candidates"`
}

type ResolveEntityResponse struct {
	Match string `json:"match"`
}

type SemanticSearchRequest struct {
	Query      string                 `json:"query"`
	Candidates []map[string]interface{} `json:"candidates"`
	TopK       int                    `json:"top_k,omitempty"`
	Threshold   float64                `json:"threshold,omitempty"`
}

type SemanticSearchResponse struct {
	Results []map[string]interface{} `json:"results"`
}

type CognifyItem struct {
	SourceID    string                 `json:"source_id"`
	SourceTable string                 `json:"source_table"`
	Content     string                 `json:"content"`
	RawData     map[string]interface{} `json:"raw_data,omitempty"`
}

type CognifyBatchRequest struct {
	Items []CognifyItem `json:"items"`
}

type CognifyResult struct {
	SourceID   string            `json:"source_id"`
	Entities   []ExtractedEntity `json:"entities,omitempty"`
	Relations  []interface{}     `json:"relations,omitempty"`
}

// SummarizeBatchRequest is the request for wisdom layer summarization
type SummarizeBatchRequest struct {
	Text string `json:"text"`
	Type string `json:"type"` // "crystallize"
}

// SummarizeBatchResponse is the response for wisdom layer summarization
type SummarizeBatchResponse struct {
	Summary  string            `json:"summary"`
	Entities []ExtractedEntity `json:"entities"`
}

// Handler implementations

func (s *AIService) extractEntities(req *server.Request, r ExtractRequest) *server.Response {
	start := time.Now()
	ctx := context.Background()

	prompt := fmt.Sprintf(`Extract entities from this conversation. Return JSON array:
[{"name": "...", "type": "Person|Organization|Concept|Metric|Location", "description": "..."}]

User Query: %s
AI Response: %s
Context: %s

Focus on:
- Named entities (people, organizations, locations)
- Concepts and topics
- Metrics and measurements
- Relationships mentioned

JSON:`, r.UserQuery, r.AIResponse, orDefault(r.Context, "None"))

	// Use default provider (auto-detects based on available API keys)
	result, err := s.llmRouter.ExtractJSON(ctx, prompt, "", "")
	if err != nil {
		s.logger.Warn("extraction failed", zap.Error(err))
		return server.JSON([]ExtractedEntity{}, 200)
	}

	// Debug: log the result map
	s.logger.Info("extraction result", zap.Any("result", result))

	entities := []ExtractedEntity{}

	// Try multiple possible keys for the entity array
	var entityArray []interface{}
	var found bool
	for _, key := range []string{"entities", "items", "results", "data"} {
		if arr, ok := result[key].([]interface{}); ok {
			entityArray = arr
			found = true
			s.logger.Info("found entity array", zap.String("key", key), zap.Int("length", len(arr)))
			break
		}
	}

	if found {
		for _, item := range entityArray {
			if entityMap, ok := item.(map[string]interface{}); ok {
				entities = append(entities, ExtractedEntity{
					Name:        getString(entityMap, "name"),
					Type:        getString(entityMap, "type"),
					Description: getString(entityMap, "description"),
					Source:      "llm",
					Confidence:  getFloat(entityMap, "confidence"),
				})
			}
		}
	} else {
		s.logger.Warn("no entity array in result", zap.Any("keys", getMapKeys(result)))
	}

	s.logger.Info("extracted entities",
		zap.Int("count", len(entities)),
		zap.Duration("duration", time.Since(start)))

	return server.JSON(entities, 200)
}

func (s *AIService) curateFacts(req *server.Request, r CurationRequest) *server.Response {
	ctx := context.Background()

	// Parse timestamps or use current time if invalid
	time1 := parseTime(r.Node1CreatedAt)
	time2 := parseTime(r.Node2CreatedAt)

	node1 := &curation.Node{
		Name:        r.Node1Name,
		Description: r.Node1Description,
		CreatedAt:   time1,
	}

	node2 := &curation.Node{
		Name:        r.Node2Name,
		Description: r.Node2Description,
		CreatedAt:   time2,
	}

	result, err := s.curation.Resolve(ctx, node1, node2)
	if err != nil {
		s.logger.Warn("curation failed", zap.Error(err))
		// Return default - favor more recent
		if time1.After(time2) {
			return server.JSON(CurationResponse{WinnerIndex: 1, Reason: "More recent"}, 200)
		}
		return server.JSON(CurationResponse{WinnerIndex: 2, Reason: "More recent"}, 200)
	}

	return server.JSON(CurationResponse{WinnerIndex: result.WinnerIndex, Reason: result.Reason}, 200)
}

func (s *AIService) synthesizeBrief(req *server.Request, r SynthesisRequest) *server.Response {
	ctx := context.Background()

	synthesizeReq := &synthesis.SynthesisRequest{
		Query:    r.Query,
		Context:  r.Context,
		Facts:    r.Facts,
		Insights: r.Insights,
		Alerts:   r.ProactiveAlerts,
	}

	result, err := s.synthesis.Synthesize(ctx, synthesizeReq)
	if err != nil {
		s.logger.Warn("synthesis failed", zap.Error(err))
		return server.JSON(SynthesisResponse{
			Brief:      "I can help with that, but I don't have specific information.",
			Confidence: 0.3,
		}, 200)
	}

	return server.JSON(SynthesisResponse{
		Brief:      result.Brief,
		Confidence: result.Confidence,
	}, 200)
}

func (s *AIService) synthesizeInsight(req *server.Request, r InsightRequest) *server.Response {
	ctx := context.Background()

	node1 := map[string]interface{}{
		"name":        r.Node1Name,
		"type":        r.Node1Type,
		"description": r.Node1Description,
	}

	node2 := map[string]interface{}{
		"name":        r.Node2Name,
		"type":        r.Node2Type,
		"description": r.Node2Description,
	}

	result, err := s.synthesis.EvaluateConnection(ctx, node1, node2, r.PathExists, r.PathLength)
	if err != nil {
		s.logger.Warn("insight evaluation failed", zap.Error(err))
		return server.JSON(InsightResponse{HasInsight: false}, 200)
	}

	return server.JSON(InsightResponse{
		HasInsight:       result.HasInsight,
		InsightType:      result.InsightType,
		Summary:          result.Summary,
		ActionSuggestion: result.ActionSuggestion,
		Confidence:       result.Confidence,
	}, 200)
}

func (s *AIService) generateResponse(req *server.Request, r GenerateRequest) *server.Response {
	ctx := context.Background()

	// Build context string
	var contextBuilder strings.Builder
	if r.Context != "" {
		contextBuilder.WriteString(r.Context)
	}
	if len(r.ProactiveAlerts) > 0 {
		if contextBuilder.Len() > 0 {
			contextBuilder.WriteString("\n\n")
		}
		contextBuilder.WriteString("Alerts:\n")
		for _, alert := range r.ProactiveAlerts {
			contextBuilder.WriteString("- ")
			contextBuilder.WriteString(alert)
			contextBuilder.WriteString("\n")
		}
	}

	genReq := &router.GenerateRequest{
		Query:       r.Query,
		Context:     contextBuilder.String(),
		Alerts:      r.ProactiveAlerts,
		UserAPIKeys: r.UserAPIKeys,
		// Don't set SystemInstruction - let the router build it using buildSystemPrompt
		// which properly includes the memory context in the prompt
	}

	result, err := s.llmRouter.Generate(ctx, genReq)
	if err != nil {
		s.logger.Warn("generation failed", zap.Error(err))
		return server.JSON(GenerateResponse{Response: "I apologize, but I'm having trouble generating a response right now."}, 500)
	}

	return server.JSON(GenerateResponse{Response: result.Content}, 200)
}

func (s *AIService) expandQuery(req *server.Request, r ExpandQueryRequest) *server.Response {
	ctx := context.Background()

	prompt := fmt.Sprintf(`Extract entity names and search terms from this query.
Return JSON: {"search_terms": ["term1", "term2"], "entity_names": ["Name1", "Name2"]}

Query: "%s"

Rules:
- search_terms: keywords to search (e.g., "metal", "favorite", "cat")
- entity_names: specific names that might be stored (e.g., "Platinum", "Luna", "Emma")
- Be thorough but concise
- Return ONLY the JSON, no explanation

JSON:`, r.Query)

	result, err := s.llmRouter.ExtractJSON(ctx, prompt, router.ProviderNVIDIA, "")
	if err != nil {
		s.logger.Warn("query expansion failed, using fallback", zap.Error(err))
		// Fallback to word extraction
		words := strings.Fields(strings.ToLower(strings.TrimSpace(r.Query)))
		searchTerms := []string{}
		for _, w := range words {
			w = strings.Trim(w, "?.!,;")
			if len(w) > 2 {
				searchTerms = append(searchTerms, w)
			}
		}
		return server.JSON(ExpandQueryResponse{
			OriginalQuery: r.Query,
			SearchTerms:   searchTerms,
			EntityNames:   []string{},
		}, 200)
	}

	searchTerms := []string{}
	entityNames := []string{}

	if st, ok := result["search_terms"].([]interface{}); ok {
		for _, item := range st {
			if s, ok := item.(string); ok {
				searchTerms = append(searchTerms, s)
			}
		}
	}

	if en, ok := result["entity_names"].([]interface{}); ok {
		for _, item := range en {
			if s, ok := item.(string); ok {
				entityNames = append(entityNames, s)
			}
		}
	}

	return server.JSON(ExpandQueryResponse{
		OriginalQuery: r.Query,
		SearchTerms:   searchTerms,
		EntityNames:   entityNames,
	}, 200)
}

func (s *AIService) extractVision(req *server.Request, r VisionExtractRequest) *server.Response {
	ctx := context.Background()

	prompt := r.Prompt
	if prompt == "" {
		prompt = `Analyze this image from a document. Extract:

1. **Type**: Is this a chart, diagram, table, or figure?
2. **Title**: What is the title or caption?
3. **Key Entities**: List all named entities (people, places, concepts, metrics)
4. **Relationships**: What relationships or connections are shown?
5. **Data Points**: Extract any numerical data or statistics
6. **Insight**: What is the main takeaway or conclusion?

Return as JSON:
{
  "type": "chart|diagram|table|figure",
  "title": "...",
  "entities": [{"name": "...", "type": "Person|Concept|Metric|Location"}],
  "relationships": [{"from": "...", "to": "...", "type": "..."}],
  "data_points": [{"label": "...", "value": "..."}],
  "insight": "..."
}`
	}

	visionReq := &router.VisionRequest{
		ImageBase64: r.ImageBase64,
		Prompt:      prompt,
	}

	rawResponse, err := s.llmRouter.GenerateVision(ctx, visionReq)
	if err != nil {
		s.logger.Warn("vision extraction failed", zap.Error(err))
		return server.JSON(VisionExtractResponse{
			RawResponse: "Failed to extract from image",
			Insights:    []string{err.Error()},
		}, 500)
	}

	// Try to parse JSON from response
	response := VisionExtractResponse{
		RawResponse: rawResponse,
	}

	// Simple JSON extraction
	jsonStart := strings.Index(rawResponse, "{")
	jsonEnd := strings.LastIndex(rawResponse, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := rawResponse[jsonStart : jsonEnd+1]
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			if entities, ok := parsed["entities"].([]interface{}); ok {
				for _, e := range entities {
					if entityMap, ok := e.(map[string]interface{}); ok {
						response.Entities = append(response.Entities, entityMap)
					}
				}
			}
			if rels, ok := parsed["relationships"].([]interface{}); ok {
				for _, r := range rels {
					if relMap, ok := r.(map[string]interface{}); ok {
						response.Relationships = append(response.Relationships, relMap)
					}
				}
			}
			if insight, ok := parsed["insight"].(string); ok && insight != "" {
				response.Insights = append(response.Insights, insight)
			}
		}
	}

	if len(response.Insights) == 0 {
		limit := min(500, len(rawResponse))
		response.Insights = []string{rawResponse[:limit]}
	}

	return server.JSON(response, 200)
}

func (s *AIService) ingestDocument(req *server.Request, r IngestRequest) *server.Response {
	ctx := context.Background()

	// Validate file if provided
	if r.ContentBase64 != "" && r.Filename != "" {
		validator := validation.DefaultConfig()
		validationResult := validator.ValidateBase64Content(r.ContentBase64, r.Filename)
		if !validationResult.Valid {
			s.logger.Warn("file validation failed",
				zap.String("error", validationResult.ErrorMessage))
			return server.JSON(map[string]any{
				"error":   "File validation failed",
				"details": validationResult.ErrorMessage,
			}, 400)
		}
	}

	var result *ingester.IngestionResult
	var err error

	if r.Text != "" {
		result, err = s.ingester.IngestText(ctx, r.Text, r.Filename)
	} else if r.ContentBase64 != "" {
		docType := r.DocumentType
		if docType == "" {
			// Infer from filename
			if strings.HasSuffix(strings.ToLower(r.Filename), ".pdf") {
				docType = "pdf"
			} else {
				docType = "text"
			}
		}
		result, err = s.ingester.IngestBase64Content(ctx, r.ContentBase64, docType, r.Filename)
	} else {
		return server.JSON(map[string]any{"error": "Either text or content_base64 is required"}, 400)
	}

	if err != nil {
		s.logger.Warn("ingestion failed", zap.Error(err))
		return server.JSON(map[string]any{"error": err.Error()}, 500)
	}

	// Convert entities
	entities := []map[string]interface{}{}
	for _, e := range result.Entities {
		entities = append(entities, map[string]interface{}{
			"name":        e.Name,
			"type":        e.EntityType,
			"description": e.Description,
			"confidence":  e.Confidence,
			"source":      e.Source,
			"count":       e.Count,
		})
	}

	// Convert chunks
	chunks := []map[string]interface{}{}
	for _, c := range result.Chunks {
		chunks = append(chunks, map[string]interface{}{
			"text":           c.Text,
			"page_number":    c.PageNumber,
			"chunk_index":    c.ChunkIndex,
			"char_count":     c.CharCount,
			"word_count":     c.WordCount,
			"is_cluster_rep": c.IsClusterRep,
		})
	}

	return server.JSON(IngestResponse{
		Entities: entities,
		Chunks:   chunks,
		Stats: map[string]interface{}{
			"pages":              result.Stats.Pages,
			"chunks":             result.Stats.TotalChunks,
			"cluster_reps":       result.Stats.ClusterReps,
			"images":             result.Stats.Images,
			"tier1_entities":     result.Stats.Tier1Entities,
			"llm_calls":          result.Stats.LLMCalls,
			"vision_calls":       result.Stats.VisionCalls,
			"total_entities":     result.Stats.TotalEntities,
			"processing_time_ms": result.Stats.ProcessingTimeMs,
			"extracted_chars":    result.Stats.ExtractedChars,
		},
		Summary:    result.Summary,
		VectorTree: result.VectorTree,
	}, 200)
}

func (s *AIService) resolveEntity(req *server.Request, r ResolveEntityRequest) *server.Response {
	ctx := context.Background()

	if len(r.Candidates) == 0 {
		return server.JSON(ResolveEntityResponse{Match: ""}, 200)
	}

	// Build candidates string
	var candidatesBuilder strings.Builder
	for _, c := range r.Candidates {
		candidatesBuilder.WriteString("- ")
		candidatesBuilder.WriteString(c)
		candidatesBuilder.WriteString("\n")
	}

	prompt := fmt.Sprintf(`You are a semantic entity judge.
Does the new entity "%s" refer to the exact same real-world concept as any of these existing entities?

Existing Candidates:
%s

Rules:
1. "Pizza" and "pizza" -> MATCH
2. "The Big Apple" and "New York City" -> MATCH
3. "Apple" (Fruit) and "Apple Inc" (Company) -> NO MATCH
4. If strict semantic match found, return the EXACT candidate name.
5. If no match or unsure, return empty string.
6. Return JSON: {"match": "Matching Candidate Name"} or {"match": ""}

JSON:`, r.Entity, candidatesBuilder.String())

	result, err := s.llmRouter.ExtractJSON(ctx, prompt, router.ProviderNVIDIA, "")
	if err != nil {
		s.logger.Warn("entity resolution failed", zap.Error(err))
		return server.JSON(ResolveEntityResponse{Match: ""}, 200)
	}

	match := getString(result, "match")

	// Verify match is actually in candidates (hallucination check)
	if match != "" {
		found := false
		for _, c := range r.Candidates {
			if c == match {
				found = true
				break
			}
		}
		if !found {
			s.logger.Warn("LLM hallucinated match not in candidates", zap.String("match", match))
			match = ""
		}
	}

	return server.JSON(ResolveEntityResponse{Match: match}, 200)
}

func (s *AIService) classifyIntent(req *server.Request, r map[string]any) *server.Response {
	ctx := context.Background()

	query := getString(r, "query")
	if query == "" {
		return server.JSON(map[string]any{"error": "query is required"}, 400)
	}

	systemPrompt := `You are an intent classifier. Classify the user message into ONE of these categories:
- GREETING: Hello, goodbye, thanks, hi, hey, how are you
- NAVIGATION: Requests to open settings, dashboard, profile, go to page
- FACT_RETRIEVAL: Questions asking for stored information (what, where, who, when, how, etc.)
- COMPLEX: Conversational messages, statements, or multi-part queries

Return ONLY the category name, nothing else.`

	genReq := &router.GenerateRequest{
		Query:           fmt.Sprintf("Classify this message: %s", query),
		SystemInstruction: systemPrompt,
		Context:         "",
	}

	result, err := s.llmRouter.Generate(ctx, genReq)
	if err != nil {
		s.logger.Warn("intent classification failed", zap.Error(err))
		return server.JSON(map[string]string{"intent": "COMPLEX"}, 200)
	}

	intent := strings.TrimSpace(strings.ToUpper(result.Content))

	// Map variations
	intentMap := map[string]string{
		"GREETING":       "GREETING",
		"NAVIGATION":     "NAVIGATION",
		"FACT_RETRIEVAL": "FACT_RETRIEVAL",
		"FACT RETRIEVAL": "FACT_RETRIEVAL",
		"FACT":           "FACT_RETRIEVAL",
		"RETRIEVAL":      "FACT_RETRIEVAL",
		"COMPLEX":        "COMPLEX",
		"CONVERSATION":   "COMPLEX",
		"CHAT":           "COMPLEX",
	}

	finalIntent := intent
	if mapped, ok := intentMap[intent]; ok {
		finalIntent = mapped
	}

	return server.JSON(map[string]string{"intent": finalIntent}, 200)
}

func (s *AIService) semanticSearch(req *server.Request, r SemanticSearchRequest) *server.Response {
	// Placeholder implementation
	return server.JSON(SemanticSearchResponse{Results: []map[string]interface{}{}}, 200)
}

func (s *AIService) cognifyBatch(req *server.Request, r CognifyBatchRequest) *server.Response {
	ctx := context.Background()

	results := []CognifyResult{}

	for _, item := range r.Items {
		// Extract entities from content
		entities := s.extractEntitiesFromContent(ctx, item.Content, item.SourceTable)

		extractedEntities := []ExtractedEntity{}
		for _, e := range entities {
			extractedEntities = append(extractedEntities, ExtractedEntity{
				Name:        e["name"],
				Type:        e["type"],
				Description: e["description"],
				Tags:        []string{item.SourceTable, "imported"},
			})
		}

		if len(extractedEntities) == 0 {
			// Use source_id as fallback
			extractedEntities = append(extractedEntities, ExtractedEntity{
				Name: item.SourceID,
				Type: "Entity",
				Tags: []string{item.SourceTable, "imported"},
			})
		}

		results = append(results, CognifyResult{
			SourceID: item.SourceID,
			Entities: extractedEntities,
		})
	}

	return server.JSON(results, 200)
}

// summarizeBatch handles wisdom layer crystallization - extracts entities from conversation
func (s *AIService) summarizeBatch(req *server.Request, r SummarizeBatchRequest) *server.Response {
	start := time.Now()
	ctx := context.Background()

	// Build extraction prompt for conversation
	prompt := fmt.Sprintf(`Analyze this conversation and extract meaningful entities and facts. Return JSON.

Conversation:
%s

INSTRUCTIONS:
1. Extract entities that represent important information shared by the user
2. Focus on: preferences, relationships, facts about the user, important events, locations, organizations
3. Each entity should have a clear name, type, and description
4. Also provide a brief summary of the conversation

Return JSON:
{
  "summary": "A brief summary of the key information shared",
  "entities": [
    {"name": "Entity Name", "type": "Person|Preference|Location|Organization|Event|Fact|Concept", "description": "What we learned about this entity"}
  ]
}

IMPORTANT:
- Skip generic greetings (hi, hello, thanks, bye)
- Only extract meaningful facts and preferences
- Be specific in descriptions

JSON:`, r.Text)

	// Use LLM to extract
	result, err := s.llmRouter.ExtractJSON(ctx, prompt, "", "")
	if err != nil {
		s.logger.Warn("summarize_batch extraction failed", zap.Error(err))
		return server.JSON(SummarizeBatchResponse{
			Summary:  "Failed to extract summary",
			Entities: []ExtractedEntity{},
		}, 200)
	}

	// Parse summary
	summary := getString(result, "summary")
	if summary == "" {
		summary = "Conversation processed"
	}

	// Parse entities
	entities := []ExtractedEntity{}
	if entityArray, ok := result["entities"].([]interface{}); ok {
		for _, item := range entityArray {
			if entityMap, ok := item.(map[string]interface{}); ok {
				name := getString(entityMap, "name")
				if name == "" {
					continue
				}
				entities = append(entities, ExtractedEntity{
					Name:        name,
					Type:        getString(entityMap, "type"),
					Description: getString(entityMap, "description"),
					Source:      "wisdom_layer",
					Confidence:  0.85,
				})
			}
		}
	}

	s.logger.Info("summarize_batch completed",
		zap.Int("entity_count", len(entities)),
		zap.String("summary_preview", summary[:min(50, len(summary))]),
		zap.Duration("duration", time.Since(start)))

	return server.JSON(SummarizeBatchResponse{
		Summary:  summary,
		Entities: entities,
	}, 200)
}

func (s *AIService) extractEntitiesFromContent(ctx context.Context, content, sourceTable string) []map[string]string {
	prompt := fmt.Sprintf(`Extract entities from this text. Return JSON array:
[{"name": "...", "type": "Person|Organization|Concept|Metric", "description": "..."}]

Text: %s

Source: %s

JSON:`, content, sourceTable)

	result, err := s.llmRouter.ExtractJSON(ctx, prompt, router.ProviderNVIDIA, "")
	if err != nil {
		s.logger.Warn("batch extraction failed", zap.Error(err))
		return []map[string]string{}
	}

	entities := []map[string]string{}
	if entityArray, ok := result["entities"].([]interface{}); ok {
		for _, item := range entityArray {
			if entityMap, ok := item.(map[string]interface{}); ok {
				entities = append(entities, map[string]string{
					"name":        getString(entityMap, "name"),
					"type":        getString(entityMap, "type"),
					"description": getString(entityMap, "description"),
				})
			}
		}
	}

	return entities
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case float32:
			return float64(val)
		}
	}
	return 0.0
}

func orDefault(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseTime(s string) time.Time {
	// Try common formats
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"01/02/2006 15:04:05",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	// Default to current time
	return time.Now()
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
