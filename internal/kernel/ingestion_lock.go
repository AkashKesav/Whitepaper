// Package kernel provides distributed locking for ingestion operations
// SECURITY: Prevents race conditions during concurrent document ingestion
package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// IngestionLock provides distributed locking for ingestion operations
// to prevent race conditions in concurrent document processing
type IngestionLock struct {
	redis     *redis.Client
	key       string
	acquired  bool
	timeout   time.Duration
	renewTick *time.Ticker
	done      chan bool
	logger    *zap.Logger
	userID    string
}

// Acquire attempts to acquire a distributed lock for ingestion
func (il *IngestionLock) Acquire(ctx context.Context) error {
	// Try to acquire with adaptive timeout
	acquired, err := il.redis.SetNX(ctx, il.key, "1", il.timeout).Result()
	if err != nil {
		return fmt.Errorf("lock acquisition failed: %w", err)
	}
	if !acquired {
		return fmt.Errorf("ingestion already in progress for user")
	}

	il.acquired = true

	// Start renewal goroutine to extend lock during long operations
	il.renewTick = time.NewTicker(il.timeout / 3)
	go func() {
		for {
			select {
			case <-il.renewTick.C:
				il.redis.Expire(ctx, il.key, il.timeout)
			case <-il.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	il.logger.Debug("Ingestion lock acquired",
		zap.String("user", il.userID),
		zap.Duration("timeout", il.timeout))

	return nil
}

// Release releases the distributed lock
func (il *IngestionLock) Release() {
	if !il.acquired {
		return
	}

	close(il.done)
	if il.renewTick != nil {
		il.renewTick.Stop()
	}

	il.redis.Del(context.Background(), il.key)
	il.acquired = false

	il.logger.Debug("Ingestion lock released",
		zap.String("user", il.userID))
}

// IngestionLockManager creates and manages locks for ingestion operations
type IngestionLockManager struct {
	redis  *redis.Client
	logger *zap.Logger
	// Default timeout for ingestion locks (30 seconds)
	// This can be adjusted based on typical ingestion times
	defaultTimeout time.Duration
}

// NewIngestionLockManager creates a new ingestion lock manager
func NewIngestionLockManager(redisClient *redis.Client, logger *zap.Logger) *IngestionLockManager {
	return &IngestionLockManager{
		redis:         redisClient,
		logger:        logger.Named("ingestion_lock"),
		defaultTimeout: 30 * time.Second,
	}
}

// AcquireUserLock acquires a lock for a user's ingestion operation
// This prevents concurrent ingestion for the same user which could cause:
// - Duplicate entities
// - Incorrect activation scores
// - Race conditions in graph updates
func (ilm *IngestionLockManager) AcquireUserLock(ctx context.Context, userID string) (*IngestionLock, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	key := fmt.Sprintf("lock:ingest:%s", userID)
	lock := &IngestionLock{
		redis:   ilm.redis,
		key:     key,
		timeout: ilm.defaultTimeout,
		done:    make(chan bool),
		logger:  ilm.logger,
		userID:  userID,
	}

	if err := lock.Acquire(ctx); err != nil {
		return nil, err
	}

	return lock, nil
}

// AcquireNamespaceLock acquires a lock for a namespace's ingestion operation
// Useful for group-level ingestion where multiple users might ingest to the same namespace
func (ilm *IngestionLockManager) AcquireNamespaceLock(ctx context.Context, namespace string) (*IngestionLock, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	key := fmt.Sprintf("lock:ingest:ns:%s", namespace)
	lock := &IngestionLock{
		redis:   ilm.redis,
		key:     key,
		timeout: ilm.defaultTimeout,
		done:    make(chan bool),
		logger:  ilm.logger,
		userID:  namespace, // Use namespace as identifier for logging
	}

	if err := lock.Acquire(ctx); err != nil {
		return nil, err
	}

	return lock, nil
}

// SetTimeout sets a custom timeout for locks created by this manager
func (ilm *IngestionLockManager) SetTimeout(timeout time.Duration) {
	ilm.defaultTimeout = timeout
	ilm.logger.Info("Ingestion lock timeout updated",
		zap.Duration("timeout", timeout))
}

// GetLockStatus checks if a lock is currently held for a user
func (ilm *IngestionLockManager) GetLockStatus(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("lock:ingest:%s", userID)
	exists, err := ilm.redis.Exists(ctx, key).Result()
	return exists > 0, err
}

// ForceRelease forcibly releases a lock for a user
// Use with caution - only for recovery scenarios
func (ilm *IngestionLockManager) ForceRelease(ctx context.Context, userID string) error {
	key := fmt.Sprintf("lock:ingest:%s", userID)
	err := ilm.redis.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to force release lock: %w", err)
	}
	ilm.logger.Info("Forcibly released ingestion lock",
		zap.String("user", userID))
	return nil
}
