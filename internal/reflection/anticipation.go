// Package reflection provides the Predictive Anticipation module.
// This module identifies recurring patterns to model and anticipate user needs.
package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// AnticipationModule detects behavioral patterns for proactive assistance
type AnticipationModule struct {
	graphClient  *graph.Client
	queryBuilder *graph.QueryBuilder
	redisClient  *redis.Client
	logger       *zap.Logger
}

// NewAnticipationModule creates a new anticipation module
func NewAnticipationModule(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	redisClient *redis.Client,
	logger *zap.Logger,
) *AnticipationModule {
	return &AnticipationModule{
		graphClient:  graphClient,
		queryBuilder: queryBuilder,
		redisClient:  redisClient,
		logger:       logger,
	}
}

// Run executes the anticipation module
func (m *AnticipationModule) Run(ctx context.Context) error {
	m.logger.Debug("Predictive Anticipation: Starting pattern detection")

	// Step 1: Analyze temporal patterns
	temporalPatterns, err := m.detectTemporalPatterns(ctx)
	if err != nil {
		m.logger.Warn("Failed to detect temporal patterns", zap.Error(err))
	}

	// Step 2: Analyze behavioral sequences
	sequencePatterns, err := m.detectSequencePatterns(ctx)
	if err != nil {
		m.logger.Warn("Failed to detect sequence patterns", zap.Error(err))
	}

	// Step 3: Update or create pattern nodes
	allPatterns := append(temporalPatterns, sequencePatterns...)
	for _, pattern := range allPatterns {
		if err := m.persistPattern(ctx, pattern); err != nil {
			m.logger.Error("Failed to persist pattern", zap.Error(err))
		}
	}

	m.logger.Info("Pattern detection completed",
		zap.Int("temporal_patterns", len(temporalPatterns)),
		zap.Int("sequence_patterns", len(sequencePatterns)))

	return nil
}

// TemporalPattern represents a time-based pattern
type TemporalPattern struct {
	Event      string
	DayOfWeek  time.Weekday
	TimeOfDay  int // Hour 0-23
	Frequency  int
	Sentiment  string
	Action     string
	Confidence float64
}

// detectTemporalPatterns finds recurring time-based patterns
func (m *AnticipationModule) detectTemporalPatterns(ctx context.Context) ([]graph.Pattern, error) {
	// PERFORMANCE: Use SCAN instead of KEYS to prevent blocking on large datasets
	// KEYS is O(N) and blocks the entire Redis database
	// SCAN is incremental and won't block
	var keys []string
	iter := m.redisClient.Scan(ctx, 0, "context:*:recent", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		// Limit to prevent excessive memory usage
		if len(keys) >= 1000 {
			m.logger.Warn("Pattern detection: reached key scan limit", zap.Int("limit", 1000))
			break
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Aggregate events by day/time
	type EventStats struct {
		Event     string
		Day       time.Weekday
		Hour      int
		Count     int
		Sentiment map[string]int
		Actions   map[string]int
	}

	eventMap := make(map[string]*EventStats)

	for _, key := range keys {
		events, err := m.redisClient.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, eventJSON := range events {
			var event graph.TranscriptEvent
			if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
				continue
			}

			day := event.Timestamp.Weekday()
			hour := event.Timestamp.Hour()

			// Extract topic from query (simplified)
			topic := extractMainTopic(event.UserQuery)
			if topic == "" {
				continue
			}

			key := fmt.Sprintf("%s-%d-%d", topic, day, hour)
			if eventMap[key] == nil {
				eventMap[key] = &EventStats{
					Event:     topic,
					Day:       day,
					Hour:      hour,
					Sentiment: make(map[string]int),
					Actions:   make(map[string]int),
				}
			}
			eventMap[key].Count++
			eventMap[key].Sentiment[event.Sentiment]++
		}
	}

	// Convert to patterns (only high-frequency ones)
	var patterns []graph.Pattern
	for _, stats := range eventMap {
		if stats.Count >= 3 { // At least 3 occurrences
			confidence := float64(stats.Count) / 10.0 // Scale confidence
			if confidence > 1.0 {
				confidence = 1.0
			}

			// Determine dominant sentiment
			dominantSentiment := "neutral"
			maxCount := 0
			for sent, count := range stats.Sentiment {
				if count > maxCount {
					maxCount = count
					dominantSentiment = sent
				}
			}

			pattern := graph.Pattern{
				Node: graph.Node{
					DType: []string{string(graph.NodeTypePattern)},
					Name:  fmt.Sprintf("Pattern: %s on %s at %d:00", stats.Event, stats.Day, stats.Hour),
				},
				PatternType:     "temporal",
				Frequency:       stats.Count,
				ConfidenceScore: confidence,
				PredictedAction: generatePredictedAction(stats.Event, dominantSentiment),
			}
			patterns = append(patterns, pattern)
		}
	}

	return patterns, nil
}

// detectSequencePatterns finds action sequences (A -> B -> C)
func (m *AnticipationModule) detectSequencePatterns(ctx context.Context) ([]graph.Pattern, error) {
	// Get recent events and look for sequences
	pattern := "context:*:recent"
	keys, err := m.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	// Track sequences
	type Sequence struct {
		Actions []string
		Count   int
	}
	sequenceMap := make(map[string]*Sequence)

	for _, key := range keys {
		events, err := m.redisClient.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		// Build action sequence for this user
		var actions []string
		for _, eventJSON := range events {
			var event graph.TranscriptEvent
			if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
				continue
			}
			action := extractMainTopic(event.UserQuery)
			if action != "" {
				actions = append(actions, action)
			}
		}

		// Look for 2-action and 3-action sequences
		for i := 0; i < len(actions)-1; i++ {
			// 2-action sequence
			seq2 := fmt.Sprintf("%s->%s", actions[i], actions[i+1])
			if sequenceMap[seq2] == nil {
				sequenceMap[seq2] = &Sequence{
					Actions: []string{actions[i], actions[i+1]},
				}
			}
			sequenceMap[seq2].Count++

			// 3-action sequence
			if i < len(actions)-2 {
				seq3 := fmt.Sprintf("%s->%s->%s", actions[i], actions[i+1], actions[i+2])
				if sequenceMap[seq3] == nil {
					sequenceMap[seq3] = &Sequence{
						Actions: []string{actions[i], actions[i+1], actions[i+2]},
					}
				}
				sequenceMap[seq3].Count++
			}
		}
	}

	// Convert to patterns
	var patterns []graph.Pattern
	for seqKey, seq := range sequenceMap {
		if seq.Count >= 2 { // At least 2 occurrences
			confidence := float64(seq.Count) / 5.0
			if confidence > 1.0 {
				confidence = 1.0
			}

			pattern := graph.Pattern{
				Node: graph.Node{
					DType: []string{string(graph.NodeTypePattern)},
					Name:  fmt.Sprintf("Sequence: %s", seqKey),
				},
				PatternType:     "sequence",
				Frequency:       seq.Count,
				ConfidenceScore: confidence,
				PredictedAction: fmt.Sprintf("After '%s', user typically does '%s'",
					seq.Actions[0], seq.Actions[len(seq.Actions)-1]),
			}
			patterns = append(patterns, pattern)
		}
	}

	return patterns, nil
}

// persistPattern saves a pattern to the graph
func (m *AnticipationModule) persistPattern(ctx context.Context, pattern graph.Pattern) error {
	// Check if pattern already exists
	existingNode, err := m.graphClient.FindNodeByName(ctx, pattern.Namespace, pattern.Name, graph.NodeTypePattern)
	if err != nil {
		return err
	}

	if existingNode != nil {
		// Update existing pattern - increment frequency
		return m.graphClient.IncrementAccessCount(ctx, existingNode.UID, graph.DefaultActivationConfig())
	}

	// Create new pattern
	node := &graph.Node{
		DType:       []string{string(graph.NodeTypePattern)},
		Name:        pattern.Name,
		Namespace:   pattern.Namespace,
		Description: pattern.PredictedAction,
		Activation:  pattern.ConfidenceScore,
		Confidence:  pattern.ConfidenceScore,
	}

	uid, err := m.graphClient.CreateNode(ctx, node)
	if err != nil {
		return err
	}

	m.logger.Debug("Created pattern node",
		zap.String("uid", uid),
		zap.String("name", pattern.Name),
		zap.Float64("confidence", pattern.ConfidenceScore))

	return nil
}

// CheckScheduledPatterns checks for patterns that should trigger based on current time/context
func (m *AnticipationModule) CheckScheduledPatterns(ctx context.Context) ([]string, error) {
	now := time.Now()
	day := now.Weekday()
	hour := now.Hour()

	// Query for temporal patterns matching current time
	query := fmt.Sprintf(`{
		patterns(func: type(Pattern)) @filter(
			anyoftext(name, "%s") AND 
			anyoftext(name, "%d:00")
		) {
			uid
			name
			description
			confidence_score
			predicted_action
		}
	}`, day.String(), hour)

	resp, err := m.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Patterns []struct {
			UID             string  `json:"uid"`
			Name            string  `json:"name"`
			Description     string  `json:"description"`
			ConfidenceScore float64 `json:"confidence_score"`
			PredictedAction string  `json:"predicted_action"`
		} `json:"patterns"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var alerts []string
	for _, p := range result.Patterns {
		if p.ConfidenceScore >= 0.7 {
			alerts = append(alerts, p.PredictedAction)
		}
	}

	return alerts, nil
}

// CreateRuleFromPattern creates an actionable rule from a high-confidence pattern
func (m *AnticipationModule) CreateRuleFromPattern(ctx context.Context, patternUID string) error {
	// Get the pattern
	patternNode, err := m.graphClient.GetNode(ctx, patternUID)
	if err != nil {
		return err
	}

	// Create a rule node linked to the pattern
	ruleNode := &graph.Node{
		DType:       []string{string(graph.NodeTypeRule)},
		Name:        fmt.Sprintf("Rule from %s", patternNode.Name),
		Description: patternNode.Description,
		Activation:  patternNode.Activation,
		Confidence:  patternNode.Confidence,
	}

	ruleUID, err := m.graphClient.CreateNode(ctx, ruleNode)
	if err != nil {
		return err
	}

	// Link rule to pattern
	return m.graphClient.CreateEdge(ctx, ruleUID, patternUID, graph.EdgeTypeDerivedFrom, graph.EdgeStatusCurrent)
}

// Helper functions

func extractMainTopic(query string) string {
	// Simplified topic extraction - in production, use NLP
	// Look for key phrases
	keywords := []string{
		"meeting", "review", "project", "deadline", "budget",
		"email", "call", "schedule", "reminder", "task",
	}

	for _, kw := range keywords {
		if containsWord(query, kw) {
			return kw
		}
	}
	return ""
}

func containsWord(text, word string) bool {
	// Simple contains check - in production, use word boundaries
	return len(text) >= len(word) &&
		(text == word ||
			containsSubstring(text, " "+word+" ") ||
			containsSubstring(text, word+" ") ||
			containsSubstring(text, " "+word))
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func generatePredictedAction(event, sentiment string) string {
	if sentiment == "negative" {
		return fmt.Sprintf("User may need support/preparation for %s", event)
	}
	return fmt.Sprintf("Anticipate user request related to %s", event)
}

// Ensure uuid is used
var _ = uuid.New
