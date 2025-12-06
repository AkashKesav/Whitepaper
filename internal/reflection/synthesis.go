// Package reflection provides the Active Synthesis module.
// This module discovers emergent insights from disparate facts - the "shower thought" capability.
package reflection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// SynthesisModule discovers new insights by connecting disparate facts
type SynthesisModule struct {
	graphClient   *graph.Client
	queryBuilder  *graph.QueryBuilder
	aiServicesURL string
	logger        *zap.Logger
}

// NewSynthesisModule creates a new synthesis module
func NewSynthesisModule(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	aiServicesURL string,
	logger *zap.Logger,
) *SynthesisModule {
	return &SynthesisModule{
		graphClient:   graphClient,
		queryBuilder:  queryBuilder,
		aiServicesURL: aiServicesURL,
		logger:        logger,
	}
}

// Run executes the synthesis module
func (m *SynthesisModule) Run(ctx context.Context) error {
	m.logger.Debug("Active Synthesis: Starting insight discovery")

	// Step 1: Get high-activation nodes (core knowledge)
	coreNodes, err := m.queryBuilder.GetHighActivationNodes(ctx, "", 0.6, 20)
	if err != nil {
		return fmt.Errorf("failed to get core nodes: %w", err)
	}

	if len(coreNodes) < 2 {
		m.logger.Debug("Not enough nodes for synthesis")
		return nil
	}

	// Step 2: Find potential connections between disparate nodes
	potentialConnections, err := m.findPotentialConnections(ctx, coreNodes)
	if err != nil {
		m.logger.Warn("Failed to find potential connections", zap.Error(err))
	}

	// Step 3: Use AI to evaluate and create insights
	for _, connection := range potentialConnections {
		insight, err := m.evaluateConnection(ctx, connection)
		if err != nil {
			m.logger.Warn("Failed to evaluate connection", zap.Error(err))
			continue
		}

		if insight != nil {
			if err := m.createInsight(ctx, insight); err != nil {
				m.logger.Error("Failed to create insight", zap.Error(err))
			} else {
				m.logger.Info("Created new insight",
					zap.String("type", insight.InsightType),
					zap.String("summary", insight.Summary))
			}
		}
	}

	return nil
}

// PotentialConnection represents a potential connection between nodes
type PotentialConnection struct {
	Node1         graph.Node
	Node2         graph.Node
	PathExists    bool
	PathLength    int
	SharedContext []string
}

// findPotentialConnections finds nodes that might be connected but aren't yet
func (m *SynthesisModule) findPotentialConnections(ctx context.Context, nodes []graph.Node) ([]PotentialConnection, error) {
	var connections []PotentialConnection

	// Check pairs of nodes for potential connections
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			// Check if path exists between nodes
			pathData, err := m.queryBuilder.FindPathBetweenNodes(ctx, nodes[i].UID, nodes[j].UID)
			if err != nil {
				continue
			}

			var pathResult struct {
				PathNodes []graph.Node `json:"path_nodes"`
			}
			if err := json.Unmarshal(pathData, &pathResult); err != nil {
				continue
			}

			connection := PotentialConnection{
				Node1:      nodes[i],
				Node2:      nodes[j],
				PathExists: len(pathResult.PathNodes) > 0,
				PathLength: len(pathResult.PathNodes),
			}

			// We're interested in both connected and unconnected pairs
			// Connected pairs might reveal emergent properties
			// Unconnected pairs might need a new connection
			connections = append(connections, connection)
		}
	}

	return connections, nil
}

// evaluateConnection uses AI to determine if a connection represents an insight
func (m *SynthesisModule) evaluateConnection(ctx context.Context, conn PotentialConnection) (*graph.Insight, error) {
	type EvaluationRequest struct {
		Node1Name  string `json:"node1_name"`
		Node1Type  string `json:"node1_type"`
		Node1Desc  string `json:"node1_description"`
		Node2Name  string `json:"node2_name"`
		Node2Type  string `json:"node2_type"`
		Node2Desc  string `json:"node2_description"`
		PathExists bool   `json:"path_exists"`
		PathLength int    `json:"path_length"`
	}

	reqBody := EvaluationRequest{
		Node1Name:  conn.Node1.Name,
		Node1Type:  string(conn.Node1.Type),
		Node1Desc:  conn.Node1.Description,
		Node2Name:  conn.Node2.Name,
		Node2Type:  string(conn.Node2.Type),
		Node2Desc:  conn.Node2.Description,
		PathExists: conn.PathExists,
		PathLength: conn.PathLength,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		m.aiServicesURL+"/synthesize-insight",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("synthesis service returned status %d", resp.StatusCode)
	}

	var result struct {
		HasInsight       bool    `json:"has_insight"`
		InsightType      string  `json:"insight_type"`
		Summary          string  `json:"summary"`
		ActionSuggestion string  `json:"action_suggestion"`
		Confidence       float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.HasInsight || result.Confidence < 0.6 {
		return nil, nil
	}

	insight := &graph.Insight{
		Node: graph.Node{
			UID:  uuid.New().String(),
			Type: graph.NodeTypeInsight,
			Name: fmt.Sprintf("Insight: %s <-> %s", conn.Node1.Name, conn.Node2.Name),
		},
		InsightType:      result.InsightType,
		SourceNodeUIDs:   []string{conn.Node1.UID, conn.Node2.UID},
		Summary:          result.Summary,
		ActionSuggestion: result.ActionSuggestion,
	}
	insight.Confidence = result.Confidence

	return insight, nil
}

// createInsight creates an insight node in the graph
func (m *SynthesisModule) createInsight(ctx context.Context, insight *graph.Insight) error {
	node := &graph.Node{
		Type:        graph.NodeTypeInsight,
		Name:        insight.Name,
		Description: insight.Summary,
		Activation:  0.8, // New insights start with high activation
		Confidence:  insight.Confidence,
	}

	uid, err := m.graphClient.CreateNode(ctx, node)
	if err != nil {
		return err
	}

	// Link insight to source nodes
	for _, sourceUID := range insight.SourceNodeUIDs {
		if err := m.graphClient.CreateEdge(ctx, uid, sourceUID, graph.EdgeTypeSynthesized, graph.EdgeStatusCurrent); err != nil {
			m.logger.Warn("Failed to link insight to source",
				zap.String("insight", uid),
				zap.String("source", sourceUID),
				zap.Error(err))
		}
	}

	return nil
}

// DiscoverAllergyConflicts specifically looks for allergy-related conflicts
// This is the "Thai food + peanut allergy" example from the whitepaper
func (m *SynthesisModule) DiscoverAllergyConflicts(ctx context.Context) ([]graph.Insight, error) {
	// Query for allergy nodes
	query := `{
		allergies(func: has(is_allergic_to)) {
			uid
			name
			is_allergic_to {
				uid
				name
			}
		}
		
		food_preferences(func: has(likes)) @filter(type(Entity)) {
			uid
			name
			likes {
				uid
				name
				description
			}
		}
	}`

	resp, err := m.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Allergies []struct {
			UID          string `json:"uid"`
			Name         string `json:"name"`
			IsAllergicTo []struct {
				UID  string `json:"uid"`
				Name string `json:"name"`
			} `json:"is_allergic_to"`
		} `json:"allergies"`
		FoodPreferences []struct {
			UID   string `json:"uid"`
			Name  string `json:"name"`
			Likes []struct {
				UID         string `json:"uid"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"likes"`
		} `json:"food_preferences"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	// Check for conflicts (simplified - in production, use ingredient lookup)
	var insights []graph.Insight
	allergyIngredients := make(map[string]string) // allergen name -> user UID

	for _, a := range result.Allergies {
		for _, allergen := range a.IsAllergicTo {
			allergyIngredients[allergen.Name] = a.UID
		}
	}

	// Check food preferences against allergies
	for _, pref := range result.FoodPreferences {
		for _, food := range pref.Likes {
			// Check if food commonly contains any allergens
			// In production, this would query a food ingredient database
			if containsAllergen(food.Name, allergyIngredients) {
				insight := graph.Insight{
					Node: graph.Node{
						Type: graph.NodeTypeInsight,
						Name: fmt.Sprintf("Allergy Risk: %s", food.Name),
					},
					InsightType:      "allergy_warning",
					Summary:          fmt.Sprintf("Food preference '%s' may contain allergens. Exercise caution.", food.Name),
					ActionSuggestion: fmt.Sprintf("When %s is mentioned, remind about potential allergy risk.", food.Name),
				}
				insight.Confidence = 0.9
				insights = append(insights, insight)
			}
		}
	}

	return insights, nil
}

// containsAllergen checks if a food commonly contains allergens (simplified)
func containsAllergen(foodName string, allergens map[string]string) bool {
	// Known food-allergen associations (simplified)
	foodAllergens := map[string][]string{
		"Thai Food":    {"peanuts", "peanut", "tree nuts", "shellfish"},
		"Thai food":    {"peanuts", "peanut", "tree nuts", "shellfish"},
		"Chinese Food": {"peanuts", "soy", "shellfish"},
		"Pad Thai":     {"peanuts", "peanut"},
		"Satay":        {"peanuts", "peanut"},
	}

	if knownAllergens, ok := foodAllergens[foodName]; ok {
		for _, allergen := range knownAllergens {
			if _, hasAllergy := allergens[allergen]; hasAllergy {
				return true
			}
		}
	}
	return false
}
