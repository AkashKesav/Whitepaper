// Package synthesis provides insight generation and synthesis for the RMK system
package synthesis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/ai/router"
	"go.uber.org/zap"
)

// Service provides advanced analytical model for insight generation
type Service struct {
	router  *router.Router
	logger  *zap.Logger
	provider router.Provider
	model    string
}

// Fact represents a memory fact
type Fact struct {
	UUID        string                 `json:"uuid"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type,omitempty"`
	DGraphType  string                 `json:"dgraph.type,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Weight      float64                `json:"weight,omitempty"`
}

// Insight represents a synthesized insight
type Insight struct {
	UUID        string    `json:"uuid,omitempty"`
	Summary     string    `json:"summary"`
	Confidence  float64   `json:"confidence"`
	InsightType string    `json:"insight_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SynthesisRequest represents a synthesis request
type SynthesisRequest struct {
	Query    string    `json:"query"`
	Context  string    `json:"context,omitempty"`
	Facts    []Fact    `json:"facts,omitempty"`
	Insights []Insight `json:"insights,omitempty"`
	Alerts   []string  `json:"alerts,omitempty"`
}

// SynthesisResponse represents a synthesis response
type SynthesisResponse struct {
	Brief      string   `json:"brief"`
	Confidence float64  `json:"confidence"`
	Sources    []string `json:"sources,omitempty"`
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	Duration   time.Duration `json:"duration"`
}

// ConnectionEvaluation represents an evaluation of node connections
type ConnectionEvaluation struct {
	HasInsight       bool    `json:"has_insight"`
	InsightType      string  `json:"insight_type"`
	Summary          string  `json:"summary"`
	ActionSuggestion string  `json:"action_suggestion"`
	Confidence       float64 `json:"confidence"`
}

// New creates a new synthesis service
func New(r *router.Router, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		router:  r,
		logger:  logger,
		provider: router.ProviderNVIDIA,
		model:    "moonshotai/kimi-k2-instruct-0905",
	}
}

// Synthesize creates a coherent brief from facts and insights
func (s *Service) Synthesize(ctx context.Context, req *SynthesisRequest) (*SynthesisResponse, error) {
	start := time.Now()

	// Format facts
	factsText := s.formatFacts(req.Facts, 10)
	insightsText := s.formatInsights(req.Insights, 5)
	alertsText := s.formatAlerts(req.Alerts, 3)

	prompt := fmt.Sprintf(`You are a memory retrieval system. Your ONLY job is to answer questions using the KNOWN FACTS below.

CRITICAL RULES:
1. If facts are provided, you MUST use them to answer
2. NEVER say "I don't have information" if facts are available
3. Quote the facts directly in your answer
4. If no facts match the query, say "I don't have that stored yet"

Query: %s

=== KNOWN FACTS (USE THESE!) ===
%s

=== INSIGHTS ===
%s

=== ALERTS ===
%s

EXAMPLE:
- If facts say "Bob: user's manager" and query is "Who is my manager?"
- Your answer MUST be: "Your manager is Bob."

Now answer the query using the facts above.

Return JSON: {"brief": "your answer using the facts", "confidence": 0.0-1.0}`,
		req.Query,
		factsText,
		insightsText,
		alertsText,
	)

	result, err := s.router.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		s.logger.Warn("synthesis LLM call failed", zap.Error(err))
		return &SynthesisResponse{
			Brief:      "I can help with that, but I don't have specific information.",
			Confidence: 0.3,
			Provider:   string(s.provider),
			Model:      s.model,
			Duration:   time.Since(start),
		}, nil
	}

	brief := "I can help with that, but I don't have specific information."
	confidence := 0.3

	if b, ok := result["brief"].(string); ok {
		brief = b
	}
	if c, ok := result["confidence"].(float64); ok {
		confidence = c
	}

	return &SynthesisResponse{
		Brief:      brief,
		Confidence: confidence,
		Provider:   string(s.provider),
		Model:      s.model,
		Duration:   time.Since(start),
	}, nil
}

// EvaluateConnection evaluates if two nodes have an emergent insight
func (s *Service) EvaluateConnection(ctx context.Context, node1, node2 map[string]interface{}, pathExists bool, pathLength int) (*ConnectionEvaluation, error) {
	name1 := getString(node1, "name")
	typ1 := getString(node1, "type")
	desc1 := getString(node1, "description")

	name2 := getString(node2, "name")
	typ2 := getString(node2, "type")
	desc2 := getString(node2, "description")

	prompt := fmt.Sprintf(`Analyze if these two pieces of information have a meaningful, non-obvious connection.

Item 1: %s (%s)
Description: %s

Item 2: %s (%s)
Description: %s

Already connected: %t (path length: %d)

Look for:
1. Potential conflicts (allergies vs food preferences)
2. Hidden dependencies
3. Causal relationships
4. Temporal patterns

Return JSON:
{
  "has_insight": true/false,
  "insight_type": "warning|opportunity|dependency|pattern",
  "summary": "brief description of the insight",
  "action_suggestion": "what to do about it",
  "confidence": 0.0-1.0
}`,
		name1, typ1, orDefault(desc1, "No description"),
		name2, typ2, orDefault(desc2, "No description"),
		pathExists, pathLength,
	)

	result, err := s.router.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		s.logger.Warn("connection evaluation LLM call failed", zap.Error(err))
		return &ConnectionEvaluation{
			HasInsight:       false,
			InsightType:      "",
			Summary:          "",
			ActionSuggestion: "",
			Confidence:       0.0,
		}, nil
	}

	eval := &ConnectionEvaluation{
		HasInsight:       getBool(result, "has_insight"),
		InsightType:      getString(result, "insight_type"),
		Summary:          getString(result, "summary"),
		ActionSuggestion: getString(result, "action_suggestion"),
		Confidence:       getFloat(result, "confidence"),
	}

	return eval, nil
}

// formatFacts formats facts into a string
func (s *Service) formatFacts(facts []Fact, max int) string {
	if len(facts) == 0 {
		return "No facts stored."
	}

	limit := len(facts)
	if max > 0 && limit > max {
		limit = max
	}

	var builder strings.Builder
	for i := 0; i < limit; i++ {
		builder.WriteString(s.formatFact(facts[i]))
		builder.WriteString("\n")
	}

	return builder.String()
}

// formatFact formats a single fact
func (s *Service) formatFact(f Fact) string {
	var builder strings.Builder

	builder.WriteString("- ")
	builder.WriteString(f.Name)

	if f.Description != "" {
		builder.WriteString(": ")
		builder.WriteString(f.Description)
	} else if f.Type != "" || f.DGraphType != "" {
		typ := f.Type
		if typ == "" {
			typ = f.DGraphType
		}
		builder.WriteString(" (")
		builder.WriteString(typ)
		builder.WriteString(")")
	}

	if len(f.Attributes) > 0 {
		var attrs []string
		for k, v := range f.Attributes {
			if v != nil && v != "" {
				attrs = append(attrs, fmt.Sprintf("%s=%v", k, v))
			}
		}
		if len(attrs) > 0 {
			builder.WriteString(" [")
			builder.WriteString(strings.Join(attrs, ", "))
			builder.WriteString("]")
		}
	}

	return builder.String()
}

// formatInsights formats insights into a string
func (s *Service) formatInsights(insights []Insight, max int) string {
	if len(insights) == 0 {
		return "None."
	}

	limit := len(insights)
	if max > 0 && limit > max {
		limit = max
	}

	var builder strings.Builder
	for i := 0; i < limit; i++ {
		builder.WriteString("- ")
		builder.WriteString(insights[i].Summary)
		builder.WriteString("\n")
	}

	return builder.String()
}

// formatAlerts formats alerts into a string
func (s *Service) formatAlerts(alerts []string, max int) string {
	if len(alerts) == 0 {
		return "None."
	}

	limit := len(alerts)
	if max > 0 && limit > max {
		limit = max
	}

	var builder strings.Builder
	for i := 0; i < limit; i++ {
		builder.WriteString("- ")
		builder.WriteString(alerts[i])
		builder.WriteString("\n")
	}

	return builder.String()
}

// SetProvider sets the LLM provider
func (s *Service) SetProvider(provider router.Provider, model string) {
	s.provider = provider
	s.model = model
}

// GetProvider returns the current LLM provider
func (s *Service) GetProvider() router.Provider {
	return s.provider
}

// GenerateSummary generates a summary from multiple facts
func (s *Service) GenerateSummary(ctx context.Context, facts []Fact, maxLength int) (string, error) {
	if len(facts) == 0 {
		return "", fmt.Errorf("no facts provided")
	}

	prompt := fmt.Sprintf(`Summarize the following facts into a concise brief (max %d words):

%s

Return JSON: {"summary": "concise summary"}`, maxLength, s.formatFacts(facts, 20))

	result, err := s.router.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		return "", err
	}

	return getString(result, "summary"), nil
}

// FindPatterns finds patterns across multiple facts
func (s *Service) FindPatterns(ctx context.Context, facts []Fact) ([]map[string]interface{}, error) {
	if len(facts) == 0 {
		return nil, fmt.Errorf("no facts provided")
	}

	prompt := fmt.Sprintf(`Analyze these facts for patterns:

%s

Look for:
1. Temporal patterns (time-based trends)
2. Categorical patterns (groupings)
3. Numerical patterns (quantities, frequencies)
4. Semantic patterns (meanings)

Return JSON: {"patterns": [{"type": "...", "description": "...", "examples": ["..."]}]}`,
		s.formatFacts(facts, 30))

	result, err := s.router.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		return nil, err
	}

	if patterns, ok := result["patterns"].([]interface{}); ok {
		// Convert to []map[string]interface{}
		var resultPatterns []map[string]interface{}
		for _, p := range patterns {
			if m, ok := p.(map[string]interface{}); ok {
				resultPatterns = append(resultPatterns, m)
			}
		}
		return resultPatterns, nil
	}

	return nil, fmt.Errorf("invalid response format")
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

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func orDefault(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

// GetStats returns service statistics
func (s *Service) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":     "synthesis",
		"provider": string(s.provider),
		"model":    s.model,
	}
}
