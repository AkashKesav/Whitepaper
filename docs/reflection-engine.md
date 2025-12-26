<<<<<<< HEAD
# Reflection Engine

The Reflection Engine performs "digital rumination" - asynchronous background processing that discovers insights, resolves contradictions, and maintains graph health.

## Overview

The engine runs four modules in parallel during each reflection cycle:

```
┌─────────────────────────────────────────────────────────────────┐
│                    REFLECTION ENGINE                             │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐│
│  │  Synthesis  │  │ Anticipation│  │  Curation   │  │Priority ││
│  │             │  │             │  │             │  │         ││
│  │ "Shower    │  │ "Pattern    │  │ "Self-      │  │"Decay/  ││
│  │  Thoughts" │  │  Detection" │  │  Healing"   │  │ Boost"  ││
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └────┬────┘│
│         │                │                │              │      │
│         └────────────────┼────────────────┼──────────────┘      │
│                          ▼                ▼                      │
│                     ┌─────────────────────────┐                 │
│                     │     Knowledge Graph     │                 │
│                     └─────────────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘
```

## Module 1: Active Synthesis

**Purpose**: Discover emergent insights from disparate facts ("shower thoughts")

### Algorithm

```
1. Get high-activation nodes (core knowledge)
2. Find potential connections between node pairs
3. For each pair:
   a. Check if path exists
   b. Evaluate connection with AI
   c. If insight discovered, create Insight node
4. Link insights to source nodes
```

### Example: Thai Food + Peanut Allergy

```
Ingestion 1: "My partner Alex loves Thai food"
  → (User) -[PARTNER_IS]-> (Alex)
  → (Alex) -[LIKES]-> (Thai Food)

Ingestion 2: "I have a severe peanut allergy"
  → (User) -[IS_ALLERGIC_TO]-> (Peanuts)

Reflection:
  → Synthesis module finds (Thai Food) and (Peanuts)
  → AI evaluation: "Thai food commonly contains peanuts"
  → Creates: (Insight: Allergy Risk) -[SYNTHESIZED_FROM]-> (Thai Food)
  → Creates: (Insight: Allergy Risk) -[SYNTHESIZED_FROM]-> (Peanuts)

Consultation: "Alex is picking up dinner"
  → Returns insight: "If Alex is getting Thai, be careful about peanuts"
```

### Insight Structure

```go
type Insight struct {
    InsightType      string   // "warning", "opportunity", "dependency"
    SourceNodeUIDs   []string // Nodes that generated this insight
    Summary          string   // Human-readable description
    ActionSuggestion string   // What to do about it
    Confidence       float64  // 0.0 - 1.0
}
```

## Module 2: Predictive Anticipation

**Purpose**: Detect temporal and behavioral patterns for proactive assistance

### Pattern Types

| Type | Description | Example |
|------|-------------|---------|
| Temporal | Time-based recurring events | "Reviews every Monday at 10am" |
| Sequence | Action chains | "After standup, requests summary" |
| Sentiment | Emotional patterns | "Frustrated before project reviews" |

### Algorithm

```
1. Scan Redis for recent conversation events
2. Aggregate by:
   - Day of week + Hour
   - Topic sequences
   - Sentiment patterns
3. If frequency >= threshold:
   a. Create Pattern node
   b. Generate predicted action
4. Check for scheduled pattern triggers
```

### Example: Monday Review Pattern

```
Week 1, 2, 3 (Mondays at 9am):
  User: "Time for Project Alpha review. I hate this meeting."
  → Sentiment: Negative
  → Topic: "Project Alpha review"

Reflection:
  → Detects pattern: (Project Alpha Review, Monday, 9am, Negative)
  → Creates: Pattern node with predicted_action: 
             "User may need preparation for Project Alpha review"

Week 4 (Monday 8am):
  → Pattern triggers
  → FEA proactively offers: "I see your Project Alpha review is today. 
                             I've summarized the last 5 action items."
```

### Pattern Structure

```go
type Pattern struct {
    PatternType     string   // "temporal", "sequence", "sentiment"
    TriggerNodes    []string // What triggers this pattern
    Frequency       int      // How many times observed
    ConfidenceScore float64  // Pattern reliability
    PredictedAction string   // What to do when triggered
}
```

## Module 3: Self-Curation

**Purpose**: Autonomously resolve contradictions and maintain consistency

### Contradiction Types

| Type | Description | Resolution |
|------|-------------|------------|
| Functional | Multiple values for single-value edge | Keep newer |
| Temporal | Outdated facts | Archive old, keep new |
| Logical | Mutually exclusive facts | AI determines winner |

### Algorithm

```
1. Check functional edge constraints
   - Query for nodes with >1 edge of functional type
2. Check temporal contradictions
   - Find facts with same name, different values
3. For each contradiction:
   a. Determine winner (timestamp > confidence > activation > AI)
   b. Archive losing node
   c. Create SUPERSEDES edge
```

### Example: Manager Change

```
January: "My manager is Bob"
  → (User) -[HAS_MANAGER {status: current, timestamp: Jan}]-> (Bob)

June: "My new manager is Alice"
  → Conflict detected: HAS_MANAGER is functional (max 1)
  
Curation:
  → Compare timestamps: June > January
  → Winner: Alice
  → Archive: (User) -[HAS_MANAGER {status: archived}]-> (Bob)
  → Create: (User) -[HAS_MANAGER {status: current}]-> (Alice)
  → Create: (Alice) -[SUPERSEDES]-> (Bob)

Query: "Who is my manager?"
  → Returns only: "Alice"
```

### Winner Determination

```go
func determineWinner(node1, node2 *Node) string {
    // Strategy 1: Prefer newer
    if node1.CreatedAt.After(node2.CreatedAt) {
        return node1.UID
    }
    
    // Strategy 2: Prefer higher confidence
    if node1.Confidence > node2.Confidence {
        return node1.UID
    }
    
    // Strategy 3: Prefer higher activation
    if node1.Activation > node2.Activation {
        return node1.UID
    }
    
    // Strategy 4: Ask AI
    return aiDetermineWinner(node1, node2)
}
```

## Module 4: Dynamic Prioritization

**Purpose**: Maintain cognitive relevance through activation management

### Activation Boost

Nodes gain activation when:
- Mentioned in conversation
- Accessed during consultation
- Related to active patterns

```go
func boost(node *Node) {
    node.Activation = min(node.Activation + boostPerAccess, maxActivation)
    node.AccessCount++
    node.LastAccessed = time.Now()
}
```

### Activation Decay

Nodes lose activation over time:

```go
func decay(node *Node) {
    daysSinceAccess := time.Since(node.LastAccessed).Hours() / 24
    
    if daysSinceAccess > 1 {
        decayFactor := math.Pow(1-decayRate, daysSinceAccess)
        node.Activation = max(node.Activation * decayFactor, minActivation)
    }
}
```

### Example: Vegan vs Birthday

```
Over several weeks:
  User mentions "vegan" 10 times
  User mentions "birthday" 1 time (January)

Prioritization:
  → (Vegan) activation: 0.95 (frequent access, no decay)
  → (Birthday) activation: 0.05 (single access, 5 months decay)

Query: "Planning a celebration, any ideas?"
  → Graph traversal starts from User
  → Finds (Vegan) immediately (low traversal cost)
  → (Birthday) is distant (high traversal cost)
  → Response focuses on vegan options, not birthday
```

### Traversal Cost

```go
func traversalCost(activation float64) float64 {
    if activation <= 0 {
        return 1000.0  // Effectively unreachable
    }
    return 1.0 / activation  // Higher activation = lower cost
}
```

## Configuration

```go
type ActivationConfig struct {
    DecayRate             float64 // 0.05 = 5% per day
    BoostPerAccess        float64 // 0.10 = 10% per access
    MinActivation         float64 // 0.01 = 1% minimum
    MaxActivation         float64 // 1.00 = 100% maximum
    CoreIdentityThreshold float64 // 0.80 = 80% for core identity
}
```

## Reflection Cycle

```go
func (e *Engine) RunCycle(ctx context.Context) error {
    // Run curation first (clean up contradictions)
    e.curation.Run(ctx)
    
    // Run remaining modules in parallel
    var wg sync.WaitGroup
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        e.prioritization.Run(ctx)
    }()
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        e.synthesis.Run(ctx)
    }()
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        e.anticipation.Run(ctx)
    }()
    
    wg.Wait()
    return nil
}
```

## Timing

| Process | Frequency | Purpose |
|---------|-----------|---------|
| Reflection Cycle | Every 5 minutes | Full rumination |
| Activation Decay | Every 1 hour | Time-based decay |
| Pattern Check | On consultation | Proactive alerts |
=======
# Reflection Engine

The Reflection Engine implements "digital rumination" - periodic processing that discovers insights, detects patterns, resolves contradictions, and manages activation priorities.

## Overview

```go
type Engine struct {
    synthesis      *SynthesisModule
    anticipation   *AnticipationModule
    curation       *CurationModule
    prioritization *PrioritizationModule
}
```

The engine runs all four modules during each reflection cycle, with curation running first followed by the other three in parallel.

---

## 1. Active Synthesis Module

**Location**: `internal/reflection/synthesis.go`

Discovers emergent insights by connecting disparate facts - the "shower thought" capability.

### Purpose
- Find nodes that might be connected but aren't linked
- Evaluate if connections represent meaningful insights
- Create insight nodes in the graph

### Example
```
Stored Facts:
  • "Alex loves Thai food"
  • "User has peanut allergy"

Synthesized Insight:
  • "Thai food commonly contains peanuts - caution advised when planning meals with Alex"
```

### Key Functions

```go
// Run executes the synthesis module
func (m *SynthesisModule) Run(ctx context.Context) error

// findPotentialConnections finds nodes that might be connected
func (m *SynthesisModule) findPotentialConnections(ctx context.Context, nodes []Node) ([]PotentialConnection, error)

// evaluateConnection uses AI to determine if connection is an insight
func (m *SynthesisModule) evaluateConnection(ctx context.Context, conn PotentialConnection) (*Insight, error)

// DiscoverAllergyConflicts specifically looks for allergy-food conflicts
func (m *SynthesisModule) DiscoverAllergyConflicts(ctx context.Context) ([]Insight, error)
```

### PotentialConnection
```go
type PotentialConnection struct {
    Node1         Node
    Node2         Node
    PathExists    bool      // Already connected in graph
    PathLength    int       // Hops between nodes
    SharedContext []string  // Common tags/themes
}
```

---

## 2. Predictive Anticipation Module

**Location**: `internal/reflection/anticipation.go`

Identifies recurring patterns to model and anticipate user needs.

### Purpose
- Detect temporal patterns (day/time-based behavior)
- Detect sequence patterns (action chains)
- Create proactive alerts for patterns

### Example
```
Detected Pattern:
  • Every Monday at 9 AM: User mentions "Project Alpha review"
  • Sentiment: Negative
  
Created Rule:
  • On Monday morning: Prepare Project Alpha status summary
```

### Key Functions

```go
// Run executes the anticipation module
func (m *AnticipationModule) Run(ctx context.Context) error

// detectTemporalPatterns finds recurring time-based patterns
func (m *AnticipationModule) detectTemporalPatterns(ctx context.Context) ([]Pattern, error)

// detectSequencePatterns finds action sequences (A -> B -> C)
func (m *AnticipationModule) detectSequencePatterns(ctx context.Context) ([]Pattern, error)

// CheckScheduledPatterns checks for patterns that should trigger now
func (m *AnticipationModule) CheckScheduledPatterns(ctx context.Context) ([]string, error)

// CreateRuleFromPattern creates actionable rule from high-confidence pattern
func (m *AnticipationModule) CreateRuleFromPattern(ctx context.Context, patternUID string) error
```

### TemporalPattern
```go
type TemporalPattern struct {
    Event      string       // What happens
    DayOfWeek  time.Weekday // When (day)
    TimeOfDay  int          // When (hour)
    Frequency  int          // How often
    Sentiment  string       // User mood
    Action     string       // Predicted action
    Confidence float64      // Detection confidence
}
```

---

## 3. Self-Curation Module

**Location**: `internal/reflection/curation.go`

Resolves contradictions and maintains graph consistency.

### Purpose
- Detect functional edge constraint violations
- Determine winning fact using temporal logic
- Archive superseded information

### Example
```
Contradiction Detected:
  • January: "My manager is Bob"
  • June: "My manager is Alice"

Resolution:
  • Archive: Bob relationship (older)
  • Keep: Alice relationship (newer)
  • Create: SUPERSEDES edge
```

### Key Functions

```go
// Run executes the curation module
func (m *CurationModule) Run(ctx context.Context) error

// checkFunctionalConstraints finds violations of functional edge constraints
func (m *CurationModule) checkFunctionalConstraints(ctx context.Context) ([]Contradiction, error)

// resolveContradiction attempts to resolve a contradiction
func (m *CurationModule) resolveContradiction(ctx context.Context, conflict Contradiction) error

// determineWinner uses temporal logic to determine which fact is correct
func (m *CurationModule) determineWinner(node1, node2 *Node) string

// ValidateGraphIntegrity performs comprehensive integrity check
func (m *CurationModule) ValidateGraphIntegrity(ctx context.Context) ([]string, error)
```

### Winner Determination Logic
1. **Recency**: Prefer more recent nodes
2. **Confidence**: Prefer higher confidence scores
3. **Activation**: Prefer higher activation levels

---

## 4. Dynamic Prioritization Module

**Location**: `internal/reflection/prioritization.go`

Implements activation boost/decay for the self-reordering graph.

### Purpose
- Boost frequently accessed nodes
- Apply decay to stale nodes
- Promote core identity nodes

### Activation Formula
```
newActivation = currentActivation * (1 - decayRate)^daysSinceAccess
```

### Key Functions

```go
// Run executes the prioritization module
func (m *PrioritizationModule) Run(ctx context.Context) error

// ApplyDecay applies activation decay to all nodes
func (m *PrioritizationModule) ApplyDecay(ctx context.Context) error

// getHighFrequencyNodes returns nodes with high access counts
func (m *PrioritizationModule) getHighFrequencyNodes(ctx context.Context) ([]Node, error)

// boostActivation increases a node's activation
func (m *PrioritizationModule) boostActivation(ctx context.Context, uid string) error

// promoteCoreIdentityNodes promotes high-activation nodes
func (m *PrioritizationModule) promoteCoreIdentityNodes(ctx context.Context) (int, error)

// CalculateTraversalCost calculates cost to traverse (inverse of activation)
func (m *PrioritizationModule) CalculateTraversalCost(activation float64) float64
```

### Core Identity
Nodes with activation >= 0.9 are considered "core identity" and are protected from aggressive decay. These represent fundamental user traits.

---

## Reflection Cycle Execution

```go
func (e *Engine) RunCycle(ctx context.Context) error {
    // 1. Curation runs first (serial)
    e.curation.Run(ctx)
    
    // 2. Other modules run in parallel
    go e.prioritization.Run(ctx)
    go e.synthesis.Run(ctx)
    go e.anticipation.Run(ctx)
    
    // Wait for completion
    // Log results
}
```

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| ReflectionInterval | 5 min | Time between cycles |
| MinBatchSize | 5 | Minimum nodes to process |
| MaxBatchSize | 50 | Maximum nodes per cycle |
| DecayRate | 0.1 | Daily activation decay |
| CoreIdentityThreshold | 0.9 | Protection threshold |
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
