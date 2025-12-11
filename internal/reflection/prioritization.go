// Package reflection provides the Dynamic Prioritization module.
// This module implements activation boost/decay for the self-reordering graph.
package reflection

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// PrioritizationModule handles dynamic graph reordering based on activation
type PrioritizationModule struct {
	graphClient  *graph.Client
	queryBuilder *graph.QueryBuilder
	config       graph.ActivationConfig
	logger       *zap.Logger
}

// NewPrioritizationModule creates a new prioritization module
func NewPrioritizationModule(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	config graph.ActivationConfig,
	logger *zap.Logger,
) *PrioritizationModule {
	return &PrioritizationModule{
		graphClient:  graphClient,
		queryBuilder: queryBuilder,
		config:       config,
		logger:       logger,
	}
}

// Run executes the prioritization module
func (m *PrioritizationModule) Run(ctx context.Context) error {
	m.logger.Debug("Dynamic Prioritization: Starting activation update")

	// Get high-frequency nodes and boost them
	highFreq, err := m.getHighFrequencyNodes(ctx)
	if err != nil {
		m.logger.Warn("Failed to get high-frequency nodes", zap.Error(err))
	}

	boosted := 0
	for _, node := range highFreq {
		if err := m.boostActivation(ctx, node.UID); err != nil {
			m.logger.Warn("Failed to boost node", zap.Error(err))
		} else {
			boosted++
		}
	}

	// Promote core identity nodes
	promoted, err := m.promoteCoreIdentityNodes(ctx)
	if err != nil {
		m.logger.Warn("Failed to promote core identity", zap.Error(err))
	}

	m.logger.Info("Prioritization completed",
		zap.Int("nodes_boosted", boosted),
		zap.Int("nodes_promoted", promoted))

	return nil
}

// ApplyDecay applies activation decay to all nodes based on time since last access
func (m *PrioritizationModule) ApplyDecay(ctx context.Context) error {
	m.logger.Debug("Applying activation decay")

	// Get all nodes with activation > minimum
	query := `{
		nodes(func: gt(activation, 0.01)) {
			uid
			activation
			last_accessed
		}
	}`

	resp, err := m.graphClient.Query(ctx, query, nil)
	if err != nil {
		return err
	}

	var result struct {
		Nodes []struct {
			UID          string    `json:"uid"`
			Activation   float64   `json:"activation"`
			LastAccessed time.Time `json:"last_accessed"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	decayed := 0
	now := time.Now()
	for _, node := range result.Nodes {
		daysSinceAccess := now.Sub(node.LastAccessed).Hours() / 24
		// For testing: apply decay to nodes not accessed in last 1 minute (originally 1 day)
		if daysSinceAccess < (1.0 / (24.0 * 60.0)) { // 1 minute in days
			continue
		}

		// Exponential decay: newActivation = activation * (1 - decayRate)^days
		decayFactor := math.Pow(1-m.config.DecayRate, daysSinceAccess)
		newActivation := node.Activation * decayFactor

		if newActivation < m.config.MinActivation {
			newActivation = m.config.MinActivation
		}

		if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, newActivation); err != nil {
			m.logger.Warn("Failed to decay node", zap.String("uid", node.UID))
		} else {
			decayed++
		}
	}

	m.logger.Info("Decay applied", zap.Int("nodes_decayed", decayed))
	return nil
}

// getHighFrequencyNodes returns nodes with high access counts
func (m *PrioritizationModule) getHighFrequencyNodes(ctx context.Context) ([]graph.Node, error) {
	query := `{
		nodes(func: gt(access_count, 5), orderdesc: access_count, first: 50) {
			uid
			name
			activation
			access_count
		}
	}`

	resp, err := m.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []graph.Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

// boostActivation increases a node's activation
func (m *PrioritizationModule) boostActivation(ctx context.Context, uid string) error {
	node, err := m.graphClient.GetNode(ctx, uid)
	if err != nil {
		return err
	}

	newActivation := node.Activation + m.config.BoostPerAccess
	if newActivation > m.config.MaxActivation {
		newActivation = m.config.MaxActivation
	}

	return m.graphClient.UpdateNodeActivation(ctx, uid, newActivation)
}

// promoteCoreIdentityNodes promotes high-activation nodes to core identity
func (m *PrioritizationModule) promoteCoreIdentityNodes(ctx context.Context) (int, error) {
	threshold := m.config.CoreIdentityThreshold

	nodes, err := m.queryBuilder.GetHighActivationNodes(ctx, "", threshold, 20)
	if err != nil {
		return 0, err
	}

	promoted := 0
	for _, node := range nodes {
		// Mark as core identity by keeping activation at max
		if node.Activation >= threshold {
			if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, m.config.MaxActivation); err != nil {
				continue
			}
			promoted++
		}
	}

	return promoted, nil
}

// CalculateTraversalCost calculates the cost to traverse to a node (inverse of activation)
func (m *PrioritizationModule) CalculateTraversalCost(activation float64) float64 {
	if activation <= 0 {
		return 1000.0 // Very high cost for inactive nodes
	}
	return 1.0 / activation
}
