package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/reflective-memory-kernel/internal/graph"
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
	UserID     string
	Groups     []string
	Clearance  int // 0=Public, 1=Internal, 2=Confidential, 3=Secret
	Attributes map[string]string
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

func (e *DefaultEngine) AddPolicy(policy Policy) error {
	e.policies = append(e.policies, policy)
	return nil
}

// Evaluate implements the core access control logic
func (e *DefaultEngine) Evaluate(ctx context.Context, user UserContext, resource *graph.Node, action Action) (Effect, error) {
	// 1. Tenant Isolation (Namespace Check)
	// If resource has a namespace, user must match it or be in a group that owns it.
	if resource.Namespace != "" {
		hasAccess := false

		// Check direct ownership
		if resource.Namespace == fmt.Sprintf("user_%s", user.UserID) {
			hasAccess = true
		}

		// Check group membership
		if !hasAccess {
			for _, group := range user.Groups {
				if resource.Namespace == fmt.Sprintf("group_%s", group) {
					hasAccess = true
					break
				}
			}
		}

		if !hasAccess {
			return EffectDeny, fmt.Errorf("namespace mismatch: resource belongs to %s", resource.Namespace)
		}
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
	// Default to Allow if no specific policies override (Open by default within namespace)
	// Or Default to Deny? For secure systems, default to Deny.
	// However, we already checked Namespace and Clearance.
	// Let's iterate policies for explicit ALLOW/DENY overrides.

	// Check for Deny first
	for _, policy := range e.policies {
		if policy.Effect == EffectDeny && e.matches(policy, user, resource, action) {
			return EffectDeny, fmt.Errorf("explicitly denied by policy %s", policy.ID)
		}
	}

	// If strict mode, we might require an explicit Allow.
	// For this system (Personal Memory Kernel), Namespace+Clearance is usually sufficient.
	// But let's allow explicit policies to grant extra access.

	return EffectAllow, nil
}

func (e *DefaultEngine) matches(policy Policy, user UserContext, resource *graph.Node, action Action) bool {
	// Check Action
	actionMatch := false
	for _, a := range policy.Actions {
		if a == action || a == "*" {
			actionMatch = true
			break
		}
	}
	if !actionMatch {
		return false
	}

	// Check Subject
	subjectMatch := false
	for _, s := range policy.Subjects {
		if s == "*" {
			subjectMatch = true
			break
		}
		if s == fmt.Sprintf("user:%s", user.UserID) {
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
	for _, r := range policy.Resources {
		if r == "*" {
			resourceMatch = true
			break
		}
		if r == fmt.Sprintf("node:%s", resource.UID) {
			resourceMatch = true
			break
		}
		if strings.HasPrefix(r, "type:") && string(resource.GetType()) == strings.TrimPrefix(r, "type:") {
			resourceMatch = true
			break
		}
	}

	return resourceMatch
}
