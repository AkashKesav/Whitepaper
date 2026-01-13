package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// Effect represents the outcome of a policy evaluation
type Effect string

const (
	EffectAllow Effect = "ALLOW"
	EffectDeny  Effect = "DENY"
)

// Action represents the operation being performed
type Action string

const (
	ActionRead   Action = "READ"
	ActionWrite  Action = "WRITE"
	ActionDelete Action = "DELETE"
	ActionAdmin  Action = "ADMIN"
)

// Policy represents an access control policy
type Policy struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Subjects    []string          `json:"subjects"`  // Users or Groups (e.g., "user:123", "group:admins")
	Resources   []string          `json:"resources"` // Node UIDs or Patterns (e.g., "node:456", "type:Financial")
	Actions     []Action          `json:"actions"`
	Effect      Effect            `json:"effect"`
	Conditions  map[string]string `json:"conditions,omitempty"` // e.g., "time_of_day": "work_hours"
}

// UserContext represents the context of the user making the request
type UserContext struct {
	UserID       string
	Groups       []string
	Clearance    int // 0=Public, 1=Internal, 2=Confidential, 3=Secret
	Attributes   map[string]string
	Authenticated bool // SECURITY: Tracks whether user is authenticated
}

// Engine defines the interface for policy evaluation
type Engine interface {
	// Evaluate checks if a user can perform an action on a resource
	Evaluate(ctx context.Context, user UserContext, resource *graph.Node, action Action) (Effect, error)

	// AddPolicy adds a new policy to the engine
	AddPolicy(policy Policy) error
}

// DefaultEngine implements a hybrid RBAC/ABAC policy engine
type DefaultEngine struct {
	policies []Policy
}

// NewEngine creates a new policy engine
func NewEngine() *DefaultEngine {
	return &DefaultEngine{
		policies: make([]Policy, 0),
	}
}

// PolicyManager integrates all policy components for the kernel
type PolicyManager struct {
	Engine        *DefaultEngine
	Store         *PolicyStore
	AuditLogger   *AuditLogger
	RateLimiter   *RateLimiter
	ContentFilter *ContentFilter
	enabled       bool
}

// PolicyManagerConfig configures the policy manager
type PolicyManagerConfig struct {
	Enabled              bool
	AuditEnabled         bool
	RateLimitEnabled     bool
	ContentFilterEnabled bool
}

// NewPolicyManager creates a fully integrated policy manager
func NewPolicyManager(config PolicyManagerConfig, graphClient *graph.Client, natsConn *nats.Conn, redisClient *redis.Client, logger *zap.Logger) *PolicyManager {
	pm := &PolicyManager{
		Engine:  NewEngine(),
		enabled: config.Enabled,
	}

	// Initialize Store
	if graphClient != nil {
		pm.Store = NewPolicyStore(graphClient, logger)
	}

	// Initialize Audit Logger
	auditConfig := AuditConfig{
		Enabled:     config.AuditEnabled,
		AsyncMode:   true,
		NATSSubject: "audit",
	}
	pm.AuditLogger = NewAuditLogger(graphClient, natsConn, logger, auditConfig)

	// Initialize Rate Limiter
	pm.RateLimiter = NewRateLimiter(redisClient, logger, config.RateLimitEnabled)

	// Initialize Content Filter
	pm.ContentFilter = NewContentFilter(logger, pm.AuditLogger, config.ContentFilterEnabled)

	return pm
}

// LoadPolicies loads policies from the store and adds them to the engine
func (pm *PolicyManager) LoadPolicies(ctx context.Context, namespace string) error {
	if pm.Store == nil {
		return nil
	}

	policies, err := pm.Store.LoadPolicies(ctx, namespace)
	if err != nil {
		return err
	}

	for _, p := range policies {
		pm.Engine.AddPolicy(p)
	}

	return nil
}

// Evaluate wraps the engine's evaluate with audit logging
func (pm *PolicyManager) Evaluate(ctx context.Context, user UserContext, resource *graph.Node, action Action) (Effect, error) {
	if !pm.enabled {
		// SECURE: When policy system is disabled, only allow same-namespace access
		if resource != nil && resource.Namespace != "" {
			expectedNamespace := fmt.Sprintf("user_%s", user.UserID)
			if resource.Namespace == expectedNamespace {
				return EffectAllow, nil
			}
			return EffectDeny, fmt.Errorf("policy system disabled - cross-namespace access denied")
		}
		return EffectAllow, nil
	}

	effect, err := pm.Engine.Evaluate(ctx, user, resource, action)

	// Log the policy decision
	if pm.AuditLogger != nil {
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		pm.AuditLogger.LogPolicyCheck(ctx, user.UserID, resource.Namespace, action, resource.UID, effect, reason)
	}

	return effect, err
}

func (e *DefaultEngine) AddPolicy(policy Policy) error {
	e.policies = append(e.policies, policy)
	return nil
}

// Evaluate implements the core access control logic
func (e *DefaultEngine) Evaluate(ctx context.Context, user UserContext, resource *graph.Node, action Action) (Effect, error) {
	// SECURITY: Require authentication for non-public resources
	// Anonymous users can only access resources explicitly tagged as public
	if !user.Authenticated && resource != nil && resource.Namespace != "" {
		isPublic := false
		for _, tag := range resource.Tags {
			if tag == "class:public" {
				isPublic = true
				break
			}
		}
		if !isPublic {
			return EffectDeny, fmt.Errorf("authentication required for resource access")
		}
	}

	// SECURITY: Validate user context consistency
	if user.Authenticated && user.UserID == "" {
		return EffectDeny, fmt.Errorf("invalid user context: authenticated but missing UserID")
	}

	// 1. Tenant Isolation (Namespace Check)
	// If resource has a namespace, user must match it or be in a group that owns it.
	// SECURITY FIX: Direct ownership grants immediate access for user's own namespace
	if resource.Namespace != "" {
		// Check direct ownership - user can always access their own namespace
		if resource.Namespace == fmt.Sprintf("user_%s", user.UserID) {
			return EffectAllow, nil // FIX: Return ALLOW immediately for user's own namespace
		}

		// Check group membership
		hasGroupAccess := false
		for _, group := range user.Groups {
			if resource.Namespace == fmt.Sprintf("group_%s", group) {
				hasGroupAccess = true
				break
			}
		}

		if !hasGroupAccess {
			return EffectDeny, fmt.Errorf("namespace mismatch: resource belongs to %s", resource.Namespace)
		}
		// FIX: Group members have implicit ALLOW access to group namespace resources
		return EffectAllow, nil
	}

	// 2. Classification/Clearance Level (ABAC)
	// Check for "class:X" tags on the node
	resourceLevel := 0
	for _, tag := range resource.Tags {
		if strings.HasPrefix(tag, "class:") {
			levelStr := strings.TrimPrefix(tag, "class:")
			switch levelStr {
			case "public":
				resourceLevel = 0
			case "internal":
				resourceLevel = 1
			case "confidential":
				resourceLevel = 2
			case "secret":
				resourceLevel = 3
			}
		}
	}

	if user.Clearance < resourceLevel {
		return EffectDeny, fmt.Errorf("insufficient clearance: user=%d, resource=%d", user.Clearance, resourceLevel)
	}

	// 3. Explicit Policy Evaluation (RBAC/Policy)
	// SECURITY: Default to Deny for secure systems - only allow if explicitly permitted
	// Require explicit ALLOW policies or namespace ownership

	// Check for Deny first (Deny overrides everything)
	for _, pol := range e.policies {
		if pol.Effect == EffectDeny {
			if e.matches(pol, user, resource, action) {
				return EffectDeny, fmt.Errorf("explicitly denied by policy %s", pol.ID)
			}
		}
	}

	// Check for explicit Allow
	for _, pol := range e.policies {
		if pol.Effect == EffectAllow {
			if e.matches(pol, user, resource, action) {
				return EffectAllow, nil
			}
		}
	}

	// Default: Deny if no explicit Allow policy matches
	// Exception: Users can always access resources in their own namespace
	if resource.Namespace == fmt.Sprintf("user_%s", user.UserID) {
		return EffectAllow, nil
	}

	return EffectDeny, fmt.Errorf("no explicit allow policy found for user=%s on resource=%s", user.UserID, resource.UID)
}

func (e *DefaultEngine) matches(policy Policy, user UserContext, resource *graph.Node, action Action) bool {
	// Check Action
	actionMatch := false
	for _, a := range policy.Actions {
		if a == action || a == "*" || string(a) == string(action) {
			actionMatch = true
			break
		}
	}
	if !actionMatch {
		return false
	}

	// Check Subject
	subjectMatch := false
	expectedSubject := fmt.Sprintf("user:%s", user.UserID)
	for _, s := range policy.Subjects {
		if s == "*" {
			subjectMatch = true
			break
		}
		if s == expectedSubject {
			subjectMatch = true
			break
		}
		for _, g := range user.Groups {
			if s == fmt.Sprintf("group:%s", g) {
				subjectMatch = true
				break
			}
		}
	}
	if !subjectMatch {
		return false
	}

	// Check Resource
	resourceMatch := false
	resourceType := string(resource.GetType())
	for _, r := range policy.Resources {
		if r == "*" {
			resourceMatch = true
			break
		}
		if r == fmt.Sprintf("node:%s", resource.UID) {
			resourceMatch = true
			break
		}
		if strings.HasPrefix(r, "type:") && resourceType == strings.TrimPrefix(r, "type:") {
			resourceMatch = true
			break
		}
	}

	return resourceMatch
}
