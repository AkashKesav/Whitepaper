// Package reflection implements the "digital rumination" engine for the Memory Kernel.
// This is the core of Phase 2: the asynchronous reflection/rumination process.
package reflection

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// Config holds configuration for the reflection engine
type Config struct {
	GraphClient      *graph.Client
	QueryBuilder     *graph.QueryBuilder
	RedisClient      *redis.Client
	AIServicesURL    string
	ActivationConfig graph.ActivationConfig

	ReflectionInterval time.Duration
	MinBatchSize       int
	MaxBatchSize       int
}

// Engine orchestrates all reflection modules
type Engine struct {
	config Config
	logger *zap.Logger

	// Reflection modules
	synthesis      *SynthesisModule
	anticipation   *AnticipationModule
	curation       *CurationModule
	prioritization *PrioritizationModule

	// Metrics
	lastCycleTime time.Time
	cycleCount    int64
	mu            sync.RWMutex
}

// NewEngine creates a new reflection engine
func NewEngine(cfg Config, logger *zap.Logger) *Engine {
	e := &Engine{
		config: cfg,
		logger: logger,
	}

	// Initialize modules
	e.synthesis = NewSynthesisModule(cfg.GraphClient, cfg.QueryBuilder, cfg.AIServicesURL, logger)
	e.anticipation = NewAnticipationModule(cfg.GraphClient, cfg.QueryBuilder, cfg.RedisClient, logger)
	e.curation = NewCurationModule(cfg.GraphClient, cfg.QueryBuilder, cfg.AIServicesURL, logger)
	e.prioritization = NewPrioritizationModule(cfg.GraphClient, cfg.QueryBuilder, cfg.ActivationConfig, logger)

	return e
}

// RunCycle executes a complete reflection cycle
func (e *Engine) RunCycle(ctx context.Context) error {
	e.mu.Lock()
	e.cycleCount++
	cycleNum := e.cycleCount
	e.mu.Unlock()

	startTime := time.Now()
	e.logger.Info("Starting reflection cycle", zap.Int64("cycle", cycleNum))

	var wg sync.WaitGroup
	errChan := make(chan error, 4)

	// Run modules in parallel where possible
	// 1. Curation should run first to clean up contradictions
	e.logger.Debug("Running curation module")
	if err := e.curation.Run(ctx); err != nil {
		e.logger.Error("Curation module failed", zap.Error(err))
		errChan <- err
	}

	// 2. Run prioritization to update activation scores
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.logger.Debug("Running prioritization module")
		if err := e.prioritization.Run(ctx); err != nil {
			e.logger.Error("Prioritization module failed", zap.Error(err))
			errChan <- err
		}
	}()

	// 3. Run synthesis to discover new insights
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.logger.Debug("Running synthesis module")
		if err := e.synthesis.Run(ctx); err != nil {
			e.logger.Error("Synthesis module failed", zap.Error(err))
			errChan <- err
		}
	}()

	// 4. Run anticipation to detect patterns
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.logger.Debug("Running anticipation module")
		if err := e.anticipation.Run(ctx); err != nil {
			e.logger.Error("Anticipation module failed", zap.Error(err))
			errChan <- err
		}
	}()

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	e.mu.Lock()
	e.lastCycleTime = time.Now()
	e.mu.Unlock()

	duration := time.Since(startTime)
	e.logger.Info("Reflection cycle completed",
		zap.Int64("cycle", cycleNum),
		zap.Duration("duration", duration),
		zap.Int("errors", len(errors)))

	if len(errors) > 0 {
		return errors[0] // Return first error
	}
	return nil
}

// ApplyDecay applies activation decay to all nodes
func (e *Engine) ApplyDecay(ctx context.Context) error {
	return e.prioritization.ApplyDecay(ctx)
}

// GetStats returns reflection engine statistics
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"cycle_count":     e.cycleCount,
		"last_cycle_time": e.lastCycleTime,
	}
}
