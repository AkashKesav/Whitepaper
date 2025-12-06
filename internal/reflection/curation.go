// Package reflection provides the Self-Curation module.
package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// CurationModule resolves contradictions and maintains graph consistency
type CurationModule struct {
	graphClient   *graph.Client
	queryBuilder  *graph.QueryBuilder
	aiServicesURL string
	logger        *zap.Logger
}

// NewCurationModule creates a new curation module
func NewCurationModule(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	aiServicesURL string,
	logger *zap.Logger,
) *CurationModule {
	return &CurationModule{
		graphClient:   graphClient,
		queryBuilder:  queryBuilder,
		aiServicesURL: aiServicesURL,
		logger:        logger,
	}
}

// Run executes the curation module
func (m *CurationModule) Run(ctx context.Context) error {
	m.logger.Debug("Self-Curation: Starting contradiction resolution")

	// Check functional edge constraints
	functionalConflicts, err := m.checkFunctionalConstraints(ctx)
	if err != nil {
		m.logger.Warn("Failed to check functional constraints", zap.Error(err))
	}

	// Resolve all conflicts
	resolved := 0
	for _, conflict := range functionalConflicts {
		if err := m.resolveContradiction(ctx, conflict); err != nil {
			m.logger.Error("Failed to resolve contradiction", zap.Error(err))
		} else {
			resolved++
		}
	}

	m.logger.Info("Curation completed",
		zap.Int("conflicts_found", len(functionalConflicts)),
		zap.Int("conflicts_resolved", resolved))

	return nil
}

// checkFunctionalConstraints finds violations of functional edge constraints
func (m *CurationModule) checkFunctionalConstraints(ctx context.Context) ([]graph.Contradiction, error) {
	var allContradictions []graph.Contradiction

	for edgeType := range graph.FunctionalEdges {
		contradictions, err := m.queryBuilder.FindPotentialContradictions(ctx, edgeType)
		if err != nil {
			continue
		}
		allContradictions = append(allContradictions, contradictions...)
	}

	return allContradictions, nil
}

// resolveContradiction attempts to resolve a contradiction
func (m *CurationModule) resolveContradiction(ctx context.Context, conflict graph.Contradiction) error {
	node1, err := m.graphClient.GetNode(ctx, conflict.NodeUID1)
	if err != nil {
		return fmt.Errorf("failed to get node1: %w", err)
	}

	node2, err := m.graphClient.GetNode(ctx, conflict.NodeUID2)
	if err != nil {
		return fmt.Errorf("failed to get node2: %w", err)
	}

	winningUID := m.determineWinner(node1, node2)
	losingUID := conflict.NodeUID1
	if losingUID == winningUID {
		losingUID = conflict.NodeUID2
	}

	// Create supersedes edge
	if err := m.graphClient.CreateEdge(ctx, winningUID, losingUID, graph.EdgeTypeSupersedes, graph.EdgeStatusCurrent); err != nil {
		m.logger.Warn("Failed to create supersedes edge", zap.Error(err))
	}

	m.logger.Info("Resolved contradiction",
		zap.String("winner", winningUID),
		zap.String("archived", losingUID))

	return nil
}

// determineWinner uses temporal logic to determine which fact is correct
func (m *CurationModule) determineWinner(node1, node2 *graph.Node) string {
	// Prefer more recent nodes
	if node1.CreatedAt.After(node2.CreatedAt) {
		return node1.UID
	} else if node2.CreatedAt.After(node1.CreatedAt) {
		return node2.UID
	}

	// Prefer higher confidence
	if node1.Confidence > node2.Confidence {
		return node1.UID
	} else if node2.Confidence > node1.Confidence {
		return node2.UID
	}

	// Prefer higher activation
	if node1.Activation > node2.Activation {
		return node1.UID
	}
	return node2.UID
}

// ValidateGraphIntegrity performs a comprehensive integrity check
func (m *CurationModule) ValidateGraphIntegrity(ctx context.Context) ([]string, error) {
	var issues []string

	orphanQuery := `{
		orphans(func: has(name)) @filter(NOT has(partner_is) AND NOT has(has_manager)) {
			uid
			name
		}
	}`

	resp, err := m.graphClient.Query(ctx, orphanQuery, nil)
	if err != nil {
		return nil, err
	}

	var orphanResult struct {
		Orphans []struct {
			UID  string `json:"uid"`
			Name string `json:"name"`
		} `json:"orphans"`
	}
	if err := json.Unmarshal(resp, &orphanResult); err != nil {
		return nil, err
	}

	for _, orphan := range orphanResult.Orphans {
		issues = append(issues, fmt.Sprintf("Orphan node: %s (%s)", orphan.Name, orphan.UID))
	}

	return issues, nil
}

var _ = time.Now // ensure time is used
