// Package graph provides the Knowledge Graph schema and client for DGraph.
// This implements the core data structures for the Reflective Memory Kernel.
package graph

import "time"

// NodeType represents the type of a node in the knowledge graph
type NodeType string

const (
	NodeTypeUser         NodeType = "User"
	NodeTypeEntity       NodeType = "Entity"
	NodeTypeEvent        NodeType = "Event"
	NodeTypeInsight      NodeType = "Insight"
	NodeTypePattern      NodeType = "Pattern"
	NodeTypePreference   NodeType = "Preference"
	NodeTypeFact         NodeType = "Fact"
	NodeTypeRule         NodeType = "Rule"
	NodeTypeGroup        NodeType = "Group"
	NodeTypeConversation NodeType = "Conversation"
)

// EdgeType represents relationship types between nodes
type EdgeType string

const (
	// Personal relationships
	EdgeTypePartnerIs    EdgeType = "PARTNER_IS"
	EdgeTypeFamilyMember EdgeType = "FAMILY_MEMBER"
	EdgeTypeFriendOf     EdgeType = "FRIEND_OF"

	// Professional relationships
	EdgeTypeHasManager EdgeType = "HAS_MANAGER"
	EdgeTypeWorksOn    EdgeType = "WORKS_ON"
	EdgeTypeWorksAt    EdgeType = "WORKS_AT"
	EdgeTypeColleague  EdgeType = "COLLEAGUE"

	// Preferences and attributes
	EdgeTypeLikes       EdgeType = "LIKES"
	EdgeTypeDislikes    EdgeType = "DISLIKES"
	EdgeTypeIsAllergic  EdgeType = "IS_ALLERGIC_TO"
	EdgeTypePrefers     EdgeType = "PREFERS"
	EdgeTypeHasInterest EdgeType = "HAS_INTEREST"

	// Causal and logical relationships
	EdgeTypeCausedBy    EdgeType = "CAUSED_BY"
	EdgeTypeBlockedBy   EdgeType = "BLOCKED_BY"
	EdgeTypeResultsIn   EdgeType = "RESULTS_IN"
	EdgeTypeContradicts EdgeType = "CONTRADICTS"

	// Temporal relationships
	EdgeTypeOccurredOn  EdgeType = "OCCURRED_ON"
	EdgeTypeScheduledAt EdgeType = "SCHEDULED_AT"

	// Meta relationships
	EdgeTypeDerivedFrom EdgeType = "DERIVED_FROM"
	EdgeTypeSynthesized EdgeType = "SYNTHESIZED_FROM"
	EdgeTypeSupersedes  EdgeType = "SUPERSEDES"

	// Knowledge relationships (User to entities)
	EdgeTypeKnows EdgeType = "KNOWS"
)

// EdgeStatus represents the current status of a relationship
type EdgeStatus string

const (
	EdgeStatusCurrent  EdgeStatus = "current"
	EdgeStatusArchived EdgeStatus = "archived"
	EdgeStatusPending  EdgeStatus = "pending"
)

// EdgeConstraint defines constraints for edge types
type EdgeConstraint struct {
	Type           EdgeType
	IsFunctional   bool // If true, only one "current" edge of this type can exist
	MaxCardinality int  // Maximum number of edges of this type (0 = unlimited)
}

// FunctionalEdges are edges where only one "current" value is valid
// e.g., a person can only have one current manager
var FunctionalEdges = map[EdgeType]bool{
	EdgeTypeHasManager: true,
	EdgeTypePartnerIs:  true,
	EdgeTypeWorksAt:    true,
}

// Node represents a node in the knowledge graph
// Node represents a node in the knowledge graph
type Node struct {
	UID         string            `json:"uid,omitempty"`
	DType       []string          `json:"dgraph.type,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`

	// Temporal metadata
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	LastAccessed time.Time `json:"last_accessed,omitempty"`

	// Activation for dynamic prioritization
	Activation  float64 `json:"activation,omitempty"`
	AccessCount int64   `json:"access_count,omitempty"`

	// Source tracking
	SourceConversationID string  `json:"source_conversation_id,omitempty"`
	Confidence           float64 `json:"confidence,omitempty"`
	Namespace            string  `json:"namespace,omitempty"` // "user_<UUID>" or "group_<UUID>"
}

// GetType returns the primary type of the node
func (n *Node) GetType() NodeType {
	if len(n.DType) > 0 {
		return NodeType(n.DType[0])
	}
	return ""
}

// SetType sets the primary type of the node
func (n *Node) SetType(t NodeType) {
	n.DType = []string{string(t)}
}

// Edge represents a relationship between nodes
type Edge struct {
	UID    string     `json:"uid,omitempty"`
	Type   EdgeType   `json:"edge_type,omitempty"`
	Status EdgeStatus `json:"status,omitempty"`

	// Source and target nodes
	FromUID string `json:"from_uid,omitempty"`
	ToUID   string `json:"to_uid,omitempty"`

	// Temporal metadata
	CreatedAt  time.Time  `json:"created_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at,omitempty"`
	ValidFrom  time.Time  `json:"valid_from,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`

	// Dynamic prioritization
	Activation    float64 `json:"activation,omitempty"`
	TraversalCost float64 `json:"traversal_cost,omitempty"`

	// Metadata
	Properties map[string]interface{} `json:"properties,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
}

// Insight represents a synthesized insight from the reflection phase
type Insight struct {
	Node
	InsightType      string   `json:"insight_type,omitempty"`
	SourceNodeUIDs   []string `json:"source_node_uids,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	ActionSuggestion string   `json:"action_suggestion,omitempty"`
}

// Pattern represents a detected behavioral pattern
type Pattern struct {
	Node
	PatternType     string   `json:"pattern_type,omitempty"`
	TriggerNodes    []string `json:"trigger_nodes,omitempty"`
	Frequency       int      `json:"frequency,omitempty"`
	ConfidenceScore float64  `json:"confidence_score,omitempty"`
	PredictedAction string   `json:"predicted_action,omitempty"`
}

// Contradiction represents a detected contradiction between facts
type Contradiction struct {
	NodeUID1   string     `json:"node_uid_1,omitempty"`
	NodeUID2   string     `json:"node_uid_2,omitempty"`
	EdgeType   EdgeType   `json:"edge_type,omitempty"`
	DetectedAt time.Time  `json:"detected_at,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Resolution string     `json:"resolution,omitempty"`
	WinningUID string     `json:"winning_uid,omitempty"`
}

// Group represents a group for shared memories
type Group struct {
	UID         string    `json:"uid,omitempty"`
	DType       []string  `json:"dgraph.type,omitempty"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedBy   *Node     `json:"created_by,omitempty"`
	Members     []Node    `json:"group_members,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// TranscriptEvent represents an ingested conversation event
type TranscriptEvent struct {
	ID                string            `json:"id,omitempty"`
	UserID            string            `json:"user_id,omitempty"`
	Namespace         string            `json:"namespace,omitempty"` // NEW: Context isolation
	ConversationID    string            `json:"conversation_id,omitempty"`
	Timestamp         time.Time         `json:"timestamp,omitempty"`
	UserQuery         string            `json:"user_query,omitempty"`
	AIResponse        string            `json:"ai_response,omitempty"`
	ExtractedEntities []ExtractedEntity `json:"extracted_entities,omitempty"`
	Sentiment         string            `json:"sentiment,omitempty"`
	Topics            []string          `json:"topics,omitempty"`
}

// ExtractedEntity represents an entity extracted from conversation
type ExtractedEntity struct {
	Name        string              `json:"name,omitempty"`
	Description string              `json:"description,omitempty"`
	Type        NodeType            `json:"type,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Attributes  map[string]string   `json:"attributes,omitempty"`
	Relations   []ExtractedRelation `json:"relations,omitempty"`
}

// ExtractedRelation represents a relationship extracted from conversation
type ExtractedRelation struct {
	Type       EdgeType          `json:"type,omitempty"`
	TargetName string            `json:"target_name,omitempty"`
	TargetType NodeType          `json:"target_type,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// ConsultationRequest represents a query from the FEA to the Memory Kernel
type ConsultationRequest struct {
	UserID          string   `json:"user_id,omitempty"`
	Namespace       string   `json:"namespace,omitempty"` // NEW: Context isolation
	Query           string   `json:"query,omitempty"`
	Context         string   `json:"context,omitempty"`
	MaxResults      int      `json:"max_results,omitempty"`
	IncludeInsights bool     `json:"include_insights,omitempty"`
	TopicFilters    []string `json:"topic_filters,omitempty"`
}

// ConsultationResponse represents the Memory Kernel's response to a query
type ConsultationResponse struct {
	RequestID        string    `json:"request_id,omitempty"`
	SynthesizedBrief string    `json:"synthesized_brief,omitempty"`
	RelevantFacts    []Node    `json:"relevant_facts,omitempty"`
	Insights         []Insight `json:"insights,omitempty"`
	Patterns         []Pattern `json:"patterns,omitempty"`
	ProactiveAlerts  []string  `json:"proactive_alerts,omitempty"`
	Confidence       float64   `json:"confidence,omitempty"`
}

// ActivationConfig configures the dynamic prioritization algorithm
type ActivationConfig struct {
	// DecayRate is the rate at which activation decays per day (0.0 - 1.0)
	DecayRate float64 `json:"decay_rate,omitempty"`

	// BoostPerAccess is the activation boost per access/mention
	BoostPerAccess float64 `json:"boost_per_access,omitempty"`

	// MinActivation is the minimum activation level before pruning consideration
	MinActivation float64 `json:"min_activation,omitempty"`

	// MaxActivation is the maximum activation level
	MaxActivation float64 `json:"max_activation,omitempty"`

	// CoreIdentityThreshold is the threshold for promoting to core identity
	CoreIdentityThreshold float64 `json:"core_identity_threshold,omitempty"`
}

// DefaultActivationConfig returns sensible defaults for activation
func DefaultActivationConfig() ActivationConfig {
	return ActivationConfig{
		DecayRate:             0.005, // 0.5% decay per day (~12% per week, gentle)
		BoostPerAccess:        0.15,  // 15% boost per access
		MinActivation:         0.01,  // 1% minimum
		MaxActivation:         1.0,   // 100% maximum
		CoreIdentityThreshold: 0.8,   // 80% for core identity
	}
}
