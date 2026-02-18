// Package curation provides fact contradiction resolution for the RMK system
package curation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/ai/router"
	"go.uber.org/zap"
)

// Node represents a memory node with facts
type Node struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Weight      float64   `json:"weight,omitempty"`
	Specificity float64   `json:"specificity,omitempty"`
}

// ResolutionResult represents the result of a contradiction resolution
type ResolutionResult struct {
	WinnerIndex int      `json:"winner_index"` // 1 or 2
	Winner      *Node    `json:"winner,omitempty"`
	Loser       *Node    `json:"loser,omitempty"`
	Reason      string   `json:"reason"`
	Confidence  float64  `json:"confidence,omitempty"`
	Method      string   `json:"method"` // "llm" or "heuristic"
	Timestamp   time.Time `json:"timestamp"`
}

// Service provides fact curation and contradiction resolution
type Service struct {
	router *router.Router
	logger *zap.Logger
}

// New creates a new curation service
func New(r *router.Router, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		router: r,
		logger: logger,
	}
}

// Resolve determines which of two contradicting facts is more reliable
func (s *Service) Resolve(ctx context.Context, node1, node2 *Node) (*ResolutionResult, error) {
	// Try LLM-based resolution first
	llmResult, err := s.resolveWithLLM(ctx, node1, node2)
	if err == nil && llmResult != nil {
		// Add metadata
		llmResult.Method = "llm"
		llmResult.Timestamp = time.Now()

		if llmResult.WinnerIndex == 1 {
			llmResult.Winner = node1
			llmResult.Loser = node2
		} else {
			llmResult.Winner = node2
			llmResult.Loser = node1
		}

		return llmResult, nil
	}

	// Fallback to heuristic-based resolution
	return s.resolveWithHeuristic(node1, node2), nil
}

// resolveWithLLM uses LLM to resolve contradictions
func (s *Service) resolveWithLLM(ctx context.Context, node1, node2 *Node) (*ResolutionResult, error) {
	prompt := fmt.Sprintf(`You are a fact verification expert. Two facts appear to contradict each other.

Fact 1:
- Name: %s
- Description: %s
- Created: %s

Fact 2:
- Name: %s
- Description: %s
- Created: %s

Determine which fact should be kept as current. Consider:
1. More recent information usually supersedes older
2. More specific information is more reliable
3. Direct statements override implications

Return JSON: {"winner_index": 1 or 2, "reason": "brief explanation"}`,
		node1.Name, node1.Description, node1.CreatedAt.Format(time.RFC3339),
		node2.Name, node2.Description, node2.CreatedAt.Format(time.RFC3339),
	)

	result, err := s.router.ExtractJSON(ctx, prompt, "", "")
	if err != nil {
		s.logger.Warn("LLM resolution failed, using heuristic", zap.Error(err))
		return nil, err
	}

	if winnerIdx, ok := result["winner_index"].(float64); ok {
		reason := "LLM decision"
		if r, ok := result["reason"].(string); ok {
			reason = r
		}

		return &ResolutionResult{
			WinnerIndex: int(winnerIdx),
			Reason:      reason,
			Confidence:  0.8,
		}, nil
	}

	return nil, fmt.Errorf("invalid LLM response format")
}

// resolveWithHeuristic uses rule-based heuristics to resolve contradictions
func (s *Service) resolveWithHeuristic(node1, node2 *Node) *ResolutionResult {
	// Calculate scores
	score1 := s.calculateNodeScore(node1)
	score2 := s.calculateNodeScore(node2)

	var winner int
	var reason string

	if score2 > score1 {
		winner = 2
		reason = "higher_composite_score"
	} else if score1 > score2 {
		winner = 1
		reason = "higher_composite_score"
	} else if node2.UpdatedAt.After(node1.UpdatedAt) {
		winner = 2
		reason = "newer_timestamp"
	} else {
		winner = 1
		reason = "default_older_stable"
	}

	return &ResolutionResult{
		WinnerIndex: winner,
		Reason:      reason,
		Confidence:  0.6,
		Method:      "heuristic",
		Timestamp:   time.Now(),
	}
}

// calculateNodeScore calculates a reliability score for a node
func (s *Service) calculateNodeScore(node *Node) float64 {
	score := 0.0

	// Recency factor (more recent = higher score)
	age := time.Since(node.CreatedAt)
	if age < 24*time.Hour {
		score += 10
	} else if age < 7*24*time.Hour {
		score += 5
	} else if age < 30*24*time.Hour {
		score += 2
	}

	// Update frequency
	if !node.UpdatedAt.IsZero() {
		updateAge := time.Since(node.UpdatedAt)
		if updateAge < 7*24*time.Hour {
			score += 5
		}
	}

	// Specificity (longer descriptions are more specific)
	descLen := len(node.Description)
	if descLen > 200 {
		score += 8
	} else if descLen > 100 {
		score += 5
	} else if descLen > 50 {
		score += 2
	}

	// Weight (if provided)
	if node.Weight > 0 {
		score += node.Weight * 10
	}

	// Specificity score (if provided)
	if node.Specificity > 0 {
		score += node.Specificity * 5
	}

	return score
}

// BatchResolve resolves multiple contradictions in parallel
func (s *Service) BatchResolve(ctx context.Context, pairs [][2]*Node) ([]*ResolutionResult, error) {
	results := make([]*ResolutionResult, len(pairs))
	errChan := make(chan error, len(pairs))

	for i, pair := range pairs {
		go func(idx int, n1, n2 *Node) {
			result, err := s.Resolve(ctx, n1, n2)
			if err != nil {
				errChan <- err
				return
			}
			results[idx] = result
			errChan <- nil
		}(i, pair[0], pair[1])
	}

	// Wait for all results
	for i := 0; i < len(pairs); i++ {
		if err := <-errChan; err != nil {
			s.logger.Warn("batch resolve error", zap.Int("index", i), zap.Error(err))
		}
	}

	return results, nil
}

// RankByReliability ranks nodes by their reliability score
func (s *Service) RankByReliability(nodes []*Node) []*Node {
	// Create a copy to avoid modifying the original
	ranked := make([]*Node, len(nodes))
	copy(ranked, nodes)

	// Sort by score
	for i := 0; i < len(ranked)-1; i++ {
		for j := i + 1; j < len(ranked); j++ {
			if s.calculateNodeScore(ranked[j]) > s.calculateNodeScore(ranked[i]) {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	return ranked
}

// FindContradictions finds potential contradictions among nodes
func (s *Service) FindContradictions(nodes []*Node) [][2]*Node {
	var contradictions [][2]*Node

	// Simple contradiction detection based on similar names
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if s.potentialContradiction(nodes[i], nodes[j]) {
				contradictions = append(contradictions, [2]*Node{nodes[i], nodes[j]})
			}
		}
	}

	return contradictions
}

// potentialContradiction checks if two nodes might contradict each other
func (s *Service) potentialContradiction(n1, n2 *Node) bool {
	// Same or similar names but different descriptions
	if n1.Name == n2.Name {
		if n1.Description != n2.Description {
			return true
		}
	}

	// Check for negation patterns
	negationWords := []string{"not", "never", "no", "none", "false", "incorrect"}
	n1HasNegation := containsAny(n1.Description, negationWords)
	n2HasNegation := containsAny(n2.Description, negationWords)

	// If one has negation and the other doesn't, they might contradict
	if n1HasNegation != n2HasNegation {
		return true
	}

	return false
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	s = strings.ToLower(s)
	for _, sub := range substrings {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// Merge merges two nodes, keeping the most reliable information
func (s *Service) Merge(ctx context.Context, node1, node2 *Node) (*Node, error) {
	result, err := s.Resolve(ctx, node1, node2)
	if err != nil {
		return nil, err
	}

	// Start with the winner
	merged := &Node{}
	if result.WinnerIndex == 1 {
		*merged = *node1
	} else {
		*merged = *node2
	}

	// Merge additional metadata
	merged.UpdatedAt = time.Now()

	// If the loser has useful info, incorporate it
	loser := node2
	if result.WinnerIndex == 2 {
		loser = node1
	}

	// Keep the longer description if it adds detail
	if len(loser.Description) > len(merged.Description) {
		merged.Description = mergeDescriptions(merged.Description, loser.Description)
	}

	return merged, nil
}

// mergeDescriptions intelligently merges two descriptions
func mergeDescriptions(primary, secondary string) string {
	// Simple implementation: keep primary if it's comprehensive
	if len(primary) > 100 {
		return primary
	}

	// Concatenate if secondary adds information
	if len(secondary) > 0 && primary != secondary {
		return primary + " " + secondary
	}

	return primary
}

// ValidateNode checks if a node contains valid information
func (s *Service) ValidateNode(node *Node) []string {
	var issues []string

	if node.Name == "" {
		issues = append(issues, "name is empty")
	}

	if node.Description == "" {
		issues = append(issues, "description is empty")
	}

	if node.CreatedAt.IsZero() {
		issues = append(issues, "created_at is not set")
	}

	return issues
}

// CalculateSimilarity calculates similarity between two nodes
func (s *Service) CalculateSimilarity(n1, n2 *Node) float64 {
	// Simple Jaccard-like similarity
	score := 0.0

	// Name similarity
	if n1.Name == n2.Name {
		score += 0.5
	} else if levenshteinSimilarity(n1.Name, n2.Name) > 0.7 {
		score += 0.3
	}

	// Description similarity
	descSim := levenshteinSimilarity(n1.Description, n2.Description)
	score += descSim * 0.5

	return score
}

// levenshteinSimilarity calculates normalized similarity (0-1)
func levenshteinSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	dist := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	len1, len2 := len(s1), len(s2)

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	// Use a smaller matrix
	if len1 < len2 {
		s1, s2 = s2, s1
		len1, len2 = len2, len1
	}

	previous := make([]int, len2+1)
	current := make([]int, len2+1)

	for i := 0; i <= len2; i++ {
		previous[i] = i
	}

	for i := 1; i <= len1; i++ {
		current[0] = i
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			current[j] = min(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}

	return previous[len2]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// GetStats returns service statistics
func (s *Service) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type": "curation",
		"methods": []string{"llm", "heuristic"},
	}
}
