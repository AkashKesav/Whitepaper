// Package reflection provides the Dynamic Prioritization module.
// This module implements activation boost/decay for the self-reordering graph.
package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// PrioritizationModule handles dynamic graph reordering based on activation
type PrioritizationModule struct {
	graphClient  *graph.Client
	queryBuilder *graph.QueryBuilder
	redisClient  *redis.Client
	config       graph.ActivationConfig
	logger       *zap.Logger
}

// NewPrioritizationModule creates a new prioritization module
func NewPrioritizationModule(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	redisClient *redis.Client,
	config graph.ActivationConfig,
	logger *zap.Logger,
) *PrioritizationModule {
	return &PrioritizationModule{
		graphClient:  graphClient,
		queryBuilder: queryBuilder,
		redisClient:  redisClient,
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
// Uses distributed locking to prevent race conditions during concurrent updates
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
		// Apply decay to nodes not accessed in last 1 day
		if daysSinceAccess < 1.0 {
			continue
		}

		// Exponential decay: newActivation = activation * (1 - decayRate)^days
		decayFactor := math.Pow(1-m.config.DecayRate, daysSinceAccess)
		newActivation := node.Activation * decayFactor

		if newActivation < m.config.MinActivation {
			newActivation = m.config.MinActivation
		}

		// Use distributed lock to prevent race conditions
		// SECURITY: Adaptive lock with 30s timeout instead of fixed 5s
		// This prevents lock expiration during high load scenarios
		if m.redisClient != nil {
			lockKey := fmt.Sprintf("lock:activation:%s", node.UID)
			adaptiveTimeout := 30 * time.Second // Increased from 5s for resilience
			lockAcquired, lockErr := m.redisClient.SetNX(ctx, lockKey, "1", adaptiveTimeout).Result()
			if lockErr != nil {
				m.logger.Warn("Failed to acquire decay lock", zap.Error(lockErr))
				continue // Skip this node
			}
			if !lockAcquired {
				// Skip if another process is updating this node
				continue
			}

			// CRITICAL: Scope lock release to this iteration only using anonymous function
			// Without this, defer would execute at function end, causing lock accumulation
			func() {
				defer func() {
					if delCmd := m.redisClient.Del(ctx, lockKey); delCmd.Err() != nil {
						m.logger.Warn("Failed to release decay lock", zap.Error(delCmd.Err()))
					}
				}()

				if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, newActivation); err != nil {
					m.logger.Warn("Failed to decay node", zap.String("uid", node.UID))
				} else {
					decayed++
				}
			}()
		} else {
			// No Redis - direct update
			if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, newActivation); err != nil {
				m.logger.Warn("Failed to decay node", zap.String("uid", node.UID))
			} else {
				decayed++
			}
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
// Uses distributed locking to prevent race conditions during concurrent updates
func (m *PrioritizationModule) boostActivation(ctx context.Context, uid string) error {
	if m.redisClient == nil {
		// Fallback to non-locked version if Redis not available
		return m.boostActivationUnsafe(ctx, uid)
	}

	// Acquire distributed lock for this specific node
	lockKey := fmt.Sprintf("lock:activation:%s", uid)
	lockAcquired, err := m.redisClient.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
	if err != nil {
		m.logger.Warn("Failed to acquire lock, proceeding unsafely", zap.Error(err))
		return m.boostActivationUnsafe(ctx, uid)
	}
	if !lockAcquired {
		// Lock already held by another process, skip this update
		m.logger.Debug("Activation update skipped due to concurrent operation", zap.String("uid", uid))
		return nil
	}
	defer m.redisClient.Del(ctx, lockKey)

	return m.boostActivationUnsafe(ctx, uid)
}

// boostActivationUnsafe performs the activation update without locking
// This is only called internally after lock acquisition
func (m *PrioritizationModule) boostActivationUnsafe(ctx context.Context, uid string) error {
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
// Uses distributed locking to prevent race conditions during concurrent updates
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
			// Use distributed lock to prevent race conditions
			if m.redisClient != nil {
				lockKey := fmt.Sprintf("lock:activation:%s", node.UID)
				lockAcquired, lockErr := m.redisClient.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
				if lockErr != nil {
					m.logger.Warn("Failed to acquire promotion lock", zap.Error(lockErr))
					continue // Skip this node
				}
				if !lockAcquired {
					// Skip if another process is updating this node
					continue
				}

				// CRITICAL: Scope lock release to this iteration only using anonymous function
				func() {
					defer func() {
						if delCmd := m.redisClient.Del(ctx, lockKey); delCmd.Err() != nil {
							m.logger.Warn("Failed to release promotion lock", zap.Error(delCmd.Err()))
						}
					}()

					if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, m.config.MaxActivation); err == nil {
						promoted++
					}
				}()
			} else {
				// No Redis - direct update
				if err := m.graphClient.UpdateNodeActivation(ctx, node.UID, m.config.MaxActivation); err == nil {
					promoted++
				}
			}
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
