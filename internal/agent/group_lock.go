// Package agent provides distributed locking for group operations
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// GroupOperationLock provides distributed locking for group operations
// to prevent race conditions in concurrent member management
type GroupOperationLock struct {
	redis      *redis.Client
	key        string
	acquired   bool
	timeout    time.Duration
	renewTick  *time.Ticker
	done       chan bool
	logger     *zap.Logger
}

// Acquire attempts to acquire a distributed lock for group operations
func (gol *GroupOperationLock) Acquire(ctx context.Context) error {
	// Try to acquire with 30-second timeout
	acquired, err := gol.redis.SetNX(ctx, gol.key, "1", gol.timeout).Result()
	if err != nil {
		return fmt.Errorf("lock acquisition failed: %w", err)
	}
	if !acquired {
		return fmt.Errorf("operation already in progress")
	}

	gol.acquired = true

	// Start renewal goroutine to extend lock during long operations
	gol.renewTick = time.NewTicker(gol.timeout / 3)
	go func() {
		for {
			select {
			case <-gol.renewTick.C:
				gol.redis.Expire(ctx, gol.key, gol.timeout)
			case <-gol.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Release releases the distributed lock
func (gol *GroupOperationLock) Release() {
	if !gol.acquired {
		return
	}

	close(gol.done)
	if gol.renewTick != nil {
		gol.renewTick.Stop()
	}

	gol.redis.Del(context.Background(), gol.key)
	gol.acquired = false
}

// GroupLockManager creates and manages locks for group operations
type GroupLockManager struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewGroupLockManager creates a new group lock manager
func NewGroupLockManager(redisClient *redis.Client, logger *zap.Logger) *GroupLockManager {
	return &GroupLockManager{
		redis:  redisClient,
		logger: logger,
	}
}

// AcquireGroupLock acquires a lock for group operations
func (glm *GroupLockManager) AcquireGroupLock(ctx context.Context, groupNamespace, operation string) (*GroupOperationLock, error) {
	key := fmt.Sprintf("lock:group:%s:%s", operation, groupNamespace)
	lock := &GroupOperationLock{
		redis:   glm.redis,
		key:     key,
		timeout: 30 * time.Second,
		done:    make(chan bool),
		logger:  glm.logger,
	}

	if err := lock.Acquire(ctx); err != nil {
		return nil, err
	}

	return lock, nil
}
