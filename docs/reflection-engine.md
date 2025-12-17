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
