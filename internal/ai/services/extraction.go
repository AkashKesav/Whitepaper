// Package services provides AI services for entity extraction and synthesis.
// This is a Go port of the Python extraction_slm.py and synthesis_slm.py.
package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/jsonx"
)

const (
	// Maximum prompt input length for security
	MaxPromptInputLength = 5000
)

var (
	// chitchatPatterns contains patterns for messages that don't need extraction
	chitchatPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(hi|hello|hey|yo|sup)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(bye|goodbye|see you|later|cya)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(thanks|thank you|thx|ty)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(ok|okay|sure|yes|no|yep|nope)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(good|great|nice|cool|awesome)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(how are you|what's up|how's it going)[\s!.?]*$`),
		regexp.MustCompile(`(?i)^(lol|haha|hehe|xd)[\s!.?]*$`),
		regexp.MustCompile(`^[\s.!?]+$`), // Just punctuation/whitespace
	}

	// injectionPatterns contains prompt injection patterns to detect
	injectionPatterns = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`(?i)(ignore|forget|disregard)\s+(all|previous|the|above|all\s+previous)\s+(instructions?|commands?|directives?|orders?|rules?|constraints?)`), "[REDACTED INSTRUCTION OVERRIDE]"},
		{regexp.MustCompile(`(?i)(override|bypass|circumvent)\s+(instructions?|commands?|rules?|security|constraints?)`), "[REDACTED OVERRIDE ATTEMPT]"},
		{regexp.MustCompile(`(?i)(you are|act as|pretend to be|simulate|roleplay as|become)\s+(a\s+)?(admin|administrator|root|god|superuser|developer|owner|system)`), "[REDACTED ROLE CHANGE]"},
		{regexp.MustCompile(`(?i)(system|assistant|ai|model):\s*`), "[REDACTED ROLE PREFIX]"},
		{regexp.MustCompile(`(?i)(show|tell|reveal|display|output|print|write|dump|export)\s+(your|the|system)\s+(prompt|instructions?|commands?|rules?|guidelines?|configuration|setup)`), "[REDACTED PROMPT LEAKAGE]"},
		{regexp.MustCompile(`(?i)(base64|rot13|caesar|cipher|encode|decode)\s*`), "[REDACTED ENCODING]"},
		{regexp.MustCompile(`(?i)(output|return|respond)\s+(only|just|nothing but|as)\s+(json|xml|yaml|html|code|script)`), "[REDACTED FORMAT OVERRIDE]"},
		{regexp.MustCompile(`(?i)([\x60]{3}[ \t]*(json|xml|python|javascript|bash|shell)|["]{3}[ \t]*(json|xml|python|javascript))`), "[REDACTED DELIMITER]"},
	}

	consecutiveNewlines = regexp.MustCompile(`\n{3,}`)
	excessWhitespace   = regexp.MustCompile(` {5,}`)
)

// Entity represents an extracted entity
type Entity struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// ExtractionResult is the result of entity extraction
type ExtractionResult struct {
	Entities []Entity `json:"entities"`
}

// SynthesisResult is the result of synthesis
type SynthesisResult struct {
	Brief      string  `json:"brief"`
	Confidence float64 `json:"confidence"`
}

// InsightResult is the result of insight evaluation
type InsightResult struct {
	HasInsight       bool    `json:"has_insight"`
	InsightType      string  `json:"insight_type"`
	Summary          string  `json:"summary"`
	ActionSuggestion string  `json:"action_suggestion"`
	Confidence       float64 `json:"confidence"`
}

// LLMClient defines the interface for LLM clients
type LLMClient interface {
	ExtractJSON(ctx context.Context, prompt string, provider, model string) (map[string]interface{}, error)
}

// ExtractionSLM handles entity extraction from conversations
type ExtractionSLM struct {
	client    LLMClient
	provider  string
	model     string
	logger    *zap.Logger
}

// NewExtractionSLM creates a new extraction service
func NewExtractionSLM(client LLMClient, logger *zap.Logger) *ExtractionSLM {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ExtractionSLM{
		client:   client,
		provider: "glm", // Use GLM for fast, accurate entity extraction
		model:    "glm-4-plus",
		logger:   logger,
	}
}

// Extract extracts entities and relationships from a conversation turn
func (e *ExtractionSLM) Extract(ctx context.Context, userQuery, aiResponse string) ([]Entity, error) {
	// OPTIMIZATION 1: Skip chitchat messages (big time saver)
	if isChitchat(userQuery) {
		e.logger.Debug("Skipping chitchat extraction",
			zap.String("query", truncateString(userQuery, 30)))
		return nil, nil
	}

	// SECURITY: Sanitize inputs to prevent prompt injection attacks
	safeQuery := sanitizePromptInput(userQuery)
	safeResponse := sanitizePromptInput(aiResponse)

	// Check if sanitization removed too much content (potential attack)
	if len(safeQuery) < len(userQuery)/2 {
		e.logger.Warn("User query heavily sanitized (possible injection attempt)",
			zap.Int("original_len", len(userQuery)),
			zap.Int("sanitized_len", len(safeQuery)))
	}

	// Build extraction prompt
	prompt := e.buildExtractionPrompt(safeQuery, safeResponse)

	// Call LLM
	result, err := e.client.ExtractJSON(ctx, prompt, e.provider, e.model)
	if err != nil {
		e.logger.Error("LLM extraction failed", zap.Error(err))
		return nil, err
	}

	// Parse result
	entities, err := parseExtractionResult(result)
	if err != nil {
		e.logger.Error("Failed to parse extraction result", zap.Error(err))
		return nil, err
	}

	e.logger.Debug("Entity extraction completed",
		zap.Int("count", len(entities)))

	return entities, nil
}

func (e *ExtractionSLM) buildExtractionPrompt(userQuery, aiResponse string) string {
	return fmt.Sprintf(`Extract entities from this conversation. Return a JSON array.

EXAMPLES:
Conversation:
User: "My favorite dessert is gulab jamun"
AI: "That sounds delicious."
Output: [{"name": "Gulab Jamun", "type": "Preference", "description": "User's favorite dessert", "tags": ["food", "dessert", "favorite"]}]

Conversation:
User: "My sister Emma lives in Boston"
AI: "I've noted that about Emma."
Output: [{"name": "Emma", "type": "Entity", "description": "User's sister", "tags": ["family", "sister"]}, {"name": "Boston", "type": "Entity", "description": "Where Emma lives", "tags": ["city", "location"]}]

Conversation:
User: "I like hiking"
AI: "Hiking is great exercise."
Output: [{"name": "Hiking", "type": "Preference", "description": "Activity user enjoys", "tags": ["hobby", "activity", "outdoors"]}]

Conversation:
User: "The weather is nice today"
AI: "Yes it is."
Output: []

NOW EXTRACT FROM:
Conversation:
User: "%s"
AI: "%s"

Output JSON array (empty [] if nothing to extract):`, userQuery, aiResponse)
}

// SynthesisSLM handles synthesis of insights and briefs from facts
type SynthesisSLM struct {
	client   LLMClient
	provider string
	model    string
	logger   *zap.Logger
}

// NewSynthesisSLM creates a new synthesis service
func NewSynthesisSLM(client LLMClient, logger *zap.Logger) *SynthesisSLM {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SynthesisSLM{
		client:   client,
		provider: "nvidia", // Use Kimi K2 for superior long-context reasoning
		model:    "moonshotai/kimi-k2-instruct-0905",
		logger:   logger,
	}
}

// Fact represents a fact from the knowledge graph
type Fact struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// Insight represents an insight from the reflection layer
type Insight struct {
	Summary string `json:"summary"`
}

// Synthesize synthesizes a coherent brief from facts and insights
func (s *SynthesisSLM) Synthesize(ctx context.Context, query string, facts []Fact, insights []Insight, alerts []string) (*SynthesisResult, error) {
	// Format facts
	factsText := s.formatFacts(facts)
	insightsText := s.formatInsights(insights)
	alertsText := s.formatAlerts(alerts)

	prompt := s.buildSynthesisPrompt(query, factsText, insightsText, alertsText)

	// Call LLM
	result, err := s.client.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		s.logger.Error("LLM synthesis failed", zap.Error(err))
		return &SynthesisResult{
			Brief:      "I can help with that, but I don't have specific information.",
			Confidence: 0.3,
		}, nil
	}

	// Parse result
	if brief, ok := result["brief"].(string); ok && brief != "" {
		confidence := 0.5
		if c, ok := result["confidence"].(float64); ok {
			confidence = c
		}
		return &SynthesisResult{
			Brief:      brief,
			Confidence: confidence,
		}, nil
	}

	return &SynthesisResult{
		Brief:      "I can help with that, but I don't have specific information.",
		Confidence: 0.3,
	}, nil
}

func (s *SynthesisSLM) formatFacts(facts []Fact) string {
	if len(facts) == 0 {
		return "No facts stored."
	}

	var parts []string
	for _, f := range facts[:min(10, len(facts))] {
		p := fmt.Sprintf("- %s", f.Name)
		if f.Description != "" {
			p += fmt.Sprintf(": %s", f.Description)
		} else if f.Type != "" {
			p += fmt.Sprintf(" (%s)", f.Type)
		}
		if len(f.Attributes) > 0 {
			var attrs []string
			for k, v := range f.Attributes {
				if v != nil {
					attrs = append(attrs, fmt.Sprintf("%s=%v", k, v))
				}
			}
			if len(attrs) > 0 {
				p += fmt.Sprintf(" [%s]", strings.Join(attrs, ", "))
			}
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "\n")
}

func (s *SynthesisSLM) formatInsights(insights []Insight) string {
	if len(insights) == 0 {
		return "None."
	}

	var parts []string
	for _, i := range insights[:min(5, len(insights))] {
		parts = append(parts, fmt.Sprintf("- %s", i.Summary))
	}
	return strings.Join(parts, "\n")
}

func (s *SynthesisSLM) formatAlerts(alerts []string) string {
	if len(alerts) == 0 {
		return "None."
	}

	var parts []string
	for _, a := range alerts[:min(3, len(alerts))] {
		parts = append(parts, fmt.Sprintf("- %s", a))
	}
	return strings.Join(parts, "\n")
}

func (s *SynthesisSLM) buildSynthesisPrompt(query, factsText, insightsText, alertsText string) string {
	return fmt.Sprintf(`You are a memory retrieval system. Your ONLY job is to answer questions using the KNOWN FACTS below.

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

Return JSON:
{"brief": "your answer using the facts", "confidence": 0.0-1.0}`, query, factsText, insightsText, alertsText)
}

// EvaluateConnection evaluates if two nodes have an emergent insight
func (s *SynthesisSLM) EvaluateConnection(ctx context.Context, node1, node2 map[string]interface{}, pathExists bool, pathLength int) (*InsightResult, error) {
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
		getString(node1, "name"),
		getString(node1, "type"),
		getString(node1, "description"),
		getString(node2, "name"),
		getString(node2, "type"),
		getString(node2, "description"),
		pathExists,
		pathLength,
	)

	result, err := s.client.ExtractJSON(ctx, prompt, s.provider, s.model)
	if err != nil {
		s.logger.Error("LLM insight evaluation failed", zap.Error(err))
		return &InsightResult{
			HasInsight:       false,
			InsightType:      "",
			Summary:          "",
			ActionSuggestion: "",
			Confidence:       0.0,
		}, nil
	}

	if hasInsight, ok := result["has_insight"].(bool); ok {
		confidence := 0.5
		if c, ok := result["confidence"].(float64); ok {
			confidence = c
		}
		return &InsightResult{
			HasInsight:       hasInsight,
			InsightType:      getString(result, "insight_type"),
			Summary:          getString(result, "summary"),
			ActionSuggestion: getString(result, "action_suggestion"),
			Confidence:       confidence,
		}, nil
	}

	return &InsightResult{
		HasInsight:       false,
		InsightType:      "",
		Summary:          "",
		ActionSuggestion: "",
		Confidence:       0.0,
	}, nil
}

// Helper functions

func isChitchat(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 3 {
		return true
	}
	for _, pattern := range chitchatPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func sanitizePromptInput(text string) string {
	if text == "" {
		return ""
	}

	// Truncate to max length
	if len(text) > MaxPromptInputLength {
		text = text[:MaxPromptInputLength] + "..."
	}

	// Remove null bytes and control characters (except newlines and tabs)
	var sanitized strings.Builder
	for _, ch := range text {
		if ch == '\n' || ch == '\t' || (ch >= 32 && ch != 127) {
			sanitized.WriteRune(ch)
		}
	}
	text = sanitized.String()

	// Detect and replace prompt injection patterns
	for _, p := range injectionPatterns {
		text = p.pattern.ReplaceAllString(text, p.replacement)
	}

	// Escape common prompt delimiters
	text = strings.ReplaceAll(text, `"""`, "\\\"\\\"\\\"")
	text = strings.ReplaceAll(text, `'''`, "\\'\\'\\'")
	text = strings.ReplaceAll(text, "```", "\\`\\`\\`")

	// Limit consecutive newlines
	text = consecutiveNewlines.ReplaceAllString(text, "\n\n")

	// Remove excessive whitespace
	text = excessWhitespace.ReplaceAllString(text, "     ")

	return strings.TrimSpace(text)
}

func parseExtractionResult(result interface{}) ([]Entity, error) {
	// Check if result is an array
	if arr, ok := result.([]interface{}); ok {
		var entities []Entity
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				entities = append(entities, Entity{
					Name:        getString(m, "name"),
					Type:        getString(m, "type"),
					Description: getString(m, "description"),
					Tags:        getStringSlice(m, "tags"),
				})
			}
		}
		return entities, nil
	}

	// Check if result is a map with an entities array
	if m, ok := result.(map[string]interface{}); ok {
		// Check if there's an entities field
		if entities, ok := m["entities"].([]Entity); ok {
			return entities, nil
		}

		// Try to parse as JSON string in result field
		if str, ok := m["result"].(string); ok {
			var entities []Entity
			if err := jsonx.UnmarshalFromString(str, &entities); err == nil {
				return entities, nil
			}
		}
	}

	return nil, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]string); ok {
			return arr
		}
		if arr, ok := v.([]interface{}); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ConfigOptions holds configuration for AI services
type ConfigOptions struct {
	ExtractionProvider string
	ExtractionModel    string
	SynthesisProvider  string
	SynthesisModel     string
	Logger             *zap.Logger
}

// DefaultConfigOptions returns sensible defaults
func DefaultConfigOptions() ConfigOptions {
	return ConfigOptions{
		ExtractionProvider: "glm",
		ExtractionModel:    "glm-4-plus",
		SynthesisProvider:  "nvidia",
		SynthesisModel:     "moonshotai/kimi-k2-instruct-0905",
		Logger:             zap.NewNop(),
	}
}
