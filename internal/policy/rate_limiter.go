package policy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitTier represents user tier for rate limiting
type RateLimitTier string

const (
	TierFree       RateLimitTier = "free"
	TierPro        RateLimitTier = "pro"
	TierEnterprise RateLimitTier = "enterprise"
	TierUnlimited  RateLimitTier = "unlimited"
)

// RateLimitConfig defines rate limit thresholds
type RateLimitConfig struct {
	RequestsPerMinute int
	RequestsPerHour   int
	RequestsPerDay    int
	BurstSize         int // Max burst above normal rate
}

// DefaultRateLimits returns default limits for each tier
func DefaultRateLimits() map[RateLimitTier]RateLimitConfig {
	return map[RateLimitTier]RateLimitConfig{
		TierFree: {
			RequestsPerMinute: 20,
			RequestsPerHour:   200,
			RequestsPerDay:    1000,
			BurstSize:         5,
		},
		TierPro: {
			RequestsPerMinute: 100,
			RequestsPerHour:   2000,
			RequestsPerDay:    20000,
			BurstSize:         20,
		},
		TierEnterprise: {
			RequestsPerMinute: 500,
			RequestsPerHour:   10000,
			RequestsPerDay:    100000,
			BurstSize:         50,
		},
		TierUnlimited: {
			RequestsPerMinute: 0, // 0 = unlimited
			RequestsPerHour:   0,
			RequestsPerDay:    0,
			BurstSize:         0,
		},
	}
}

// RateLimiter provides rate limiting functionality using Redis
type RateLimiter struct {
	redis   *redis.Client
	logger  *zap.Logger
	limits  map[RateLimitTier]RateLimitConfig
	enabled bool
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed      bool
	Remaining    int
	ResetAt      time.Time
	RetryAfter   time.Duration
	CurrentCount int
	Limit        int
	LimitWindow  string // "minute", "hour", "day"
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, logger *zap.Logger, enabled bool) *RateLimiter {
	return &RateLimiter{
		redis:   redisClient,
		logger:  logger,
		limits:  DefaultRateLimits(),
		enabled: enabled,
	}
}

// SetLimits allows customizing rate limits
func (rl *RateLimiter) SetLimits(tier RateLimitTier, config RateLimitConfig) {
	rl.limits[tier] = config
}

// Allow checks if a request should be allowed and increments the counter
func (rl *RateLimiter) Allow(ctx context.Context, userID string, tier RateLimitTier, endpoint string) (*RateLimitResult, error) {
	if !rl.enabled || rl.redis == nil {
		return &RateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	config, ok := rl.limits[tier]
	if !ok {
		config = rl.limits[TierFree] // Default to free tier
	}

	// Check unlimited tier
	if config.RequestsPerMinute == 0 && config.RequestsPerHour == 0 && config.RequestsPerDay == 0 {
		return &RateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	now := time.Now()

	// Check each window (minute, hour, day) - deny if any exceed
	windows := []struct {
		name     string
		duration time.Duration
		limit    int
	}{
		{"minute", time.Minute, config.RequestsPerMinute},
		{"hour", time.Hour, config.RequestsPerHour},
		{"day", 24 * time.Hour, config.RequestsPerDay},
	}

	for _, w := range windows {
		if w.limit == 0 {
			continue // Skip if unlimited for this window
		}

		result, err := rl.checkWindow(ctx, userID, endpoint, w.name, w.duration, w.limit, now)
		if err != nil {
			rl.logger.Warn("Rate limit check failed", zap.Error(err), zap.String("window", w.name))
			continue // Fail open on errors
		}

		if !result.Allowed {
			return result, nil
		}
	}

	// All windows passed - increment counters
	for _, w := range windows {
		if w.limit == 0 {
			continue
		}
		rl.incrementCounter(ctx, userID, endpoint, w.name, w.duration)
	}

	// Return result for the most restrictive window (minute)
	return &RateLimitResult{
		Allowed:     true,
		Remaining:   config.RequestsPerMinute - 1,
		LimitWindow: "minute",
		Limit:       config.RequestsPerMinute,
	}, nil
}

// checkWindow checks rate limit for a specific time window
func (rl *RateLimiter) checkWindow(ctx context.Context, userID, endpoint, windowName string, duration time.Duration, limit int, now time.Time) (*RateLimitResult, error) {
	key := rl.buildKey(userID, endpoint, windowName, now, duration)

	countStr, err := rl.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist - request is allowed
		return &RateLimitResult{
			Allowed:      true,
			CurrentCount: 0,
			Remaining:    limit,
			Limit:        limit,
			LimitWindow:  windowName,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	count, _ := strconv.Atoi(countStr)

	if count >= limit {
		// Calculate when the window resets
		resetAt := rl.calculateResetTime(now, duration)
		retryAfter := resetAt.Sub(now)

		return &RateLimitResult{
			Allowed:      false,
			CurrentCount: count,
			Remaining:    0,
			Limit:        limit,
			LimitWindow:  windowName,
			ResetAt:      resetAt,
			RetryAfter:   retryAfter,
		}, nil
	}

	return &RateLimitResult{
		Allowed:      true,
		CurrentCount: count,
		Remaining:    limit - count,
		Limit:        limit,
		LimitWindow:  windowName,
	}, nil
}

// incrementCounter increments the counter for a time window
func (rl *RateLimiter) incrementCounter(ctx context.Context, userID, endpoint, windowName string, duration time.Duration) {
	key := rl.buildKey(userID, endpoint, windowName, time.Now(), duration)

	pipe := rl.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, duration)
	pipe.Exec(ctx)
}

// buildKey constructs the Redis key for rate limiting
func (rl *RateLimiter) buildKey(userID, endpoint, windowName string, now time.Time, duration time.Duration) string {
	// Use window-aligned timestamps for consistent bucket boundaries
	var windowStart int64
	switch windowName {
	case "minute":
		windowStart = now.Truncate(time.Minute).Unix()
	case "hour":
		windowStart = now.Truncate(time.Hour).Unix()
	case "day":
		windowStart = now.Truncate(24 * time.Hour).Unix()
	default:
		windowStart = now.Unix()
	}

	return fmt.Sprintf("ratelimit:%s:%s:%s:%d", userID, endpoint, windowName, windowStart)
}

// calculateResetTime calculates when the rate limit window resets
func (rl *RateLimiter) calculateResetTime(now time.Time, duration time.Duration) time.Time {
	return now.Truncate(duration).Add(duration)
}

// GetStatus returns the current rate limit status for a user
func (rl *RateLimiter) GetStatus(ctx context.Context, userID string, tier RateLimitTier) (map[string]*RateLimitResult, error) {
	if !rl.enabled || rl.redis == nil {
		return nil, nil
	}

	config := rl.limits[tier]
	now := time.Now()

	status := make(map[string]*RateLimitResult)

	windows := []struct {
		name     string
		duration time.Duration
		limit    int
	}{
		{"minute", time.Minute, config.RequestsPerMinute},
		{"hour", time.Hour, config.RequestsPerHour},
		{"day", 24 * time.Hour, config.RequestsPerDay},
	}

	for _, w := range windows {
		if w.limit == 0 {
			continue
		}
		result, _ := rl.checkWindow(ctx, userID, "*", w.name, w.duration, w.limit, now)
		status[w.name] = result
	}

	return status, nil
}

// Reset clears rate limit counters for a user
func (rl *RateLimiter) Reset(ctx context.Context, userID string) error {
	if rl.redis == nil {
		return nil
	}

	pattern := fmt.Sprintf("ratelimit:%s:*", userID)
	iter := rl.redis.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		rl.redis.Del(ctx, iter.Val())
	}

	return iter.Err()
}

// RateLimitMiddleware provides HTTP middleware for rate limiting
type RateLimitMiddleware struct {
	limiter     *RateLimiter
	getTier     func(userID string) RateLimitTier
	getUserID   func(ctx context.Context) string
	auditLogger *AuditLogger
}

// NewRateLimitMiddleware creates rate limit middleware
func NewRateLimitMiddleware(limiter *RateLimiter, getTier func(string) RateLimitTier, getUserID func(context.Context) string, auditLogger *AuditLogger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter:     limiter,
		getTier:     getTier,
		getUserID:   getUserID,
		auditLogger: auditLogger,
	}
}

// Check checks rate limit for the current request context
func (m *RateLimitMiddleware) Check(ctx context.Context, endpoint string) (*RateLimitResult, error) {
	userID := m.getUserID(ctx)
	if userID == "" {
		return &RateLimitResult{Allowed: true}, nil
	}

	tier := m.getTier(userID)
	result, err := m.limiter.Allow(ctx, userID, tier, endpoint)

	// Log rate limit denials
	if result != nil && !result.Allowed && m.auditLogger != nil {
		m.auditLogger.Log(ctx, AuditEvent{
			EventType: AuditEventAccess,
			UserID:    userID,
			Action:    "RATE_LIMITED",
			Resource:  endpoint,
			Effect:    EffectDeny,
			Reason:    fmt.Sprintf("Rate limit exceeded for %s window", result.LimitWindow),
		})
	}

	return result, err
}
