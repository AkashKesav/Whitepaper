# Knowledge Graph

The Knowledge Graph is the persistent memory store built on DGraph. It stores entities, relationships, insights, and patterns with dynamic activation scores.

## Schema Overview

### Node Types

| Type | Description | Example |
|------|-------------|---------|
| `User` | The user entity | Central node for all user knowledge |
| `Entity` | People, places, things | "Alex", "Project Alpha", "Thai Food" |
| `Fact` | Atomic facts about the world | "User is vegan" |
| `Event` | Time-bound occurrences | "Project Alpha review on Monday" |
| `Insight` | Synthesized discoveries | "Allergy risk with Thai food" |
| `Pattern` | Behavioral patterns | "Requests brief before meetings" |
| `Preference` | User preferences | "Prefers email over calls" |
| `Rule` | Actionable rules from patterns | "Prepare brief on Monday mornings" |

### Edge Types

#### Personal Relationships
```
PARTNER_IS      - Romantic partner (functional: max 1 current)
FAMILY_MEMBER   - Family relationships
FRIEND_OF       - Friendship connections
```

#### Professional Relationships
```
HAS_MANAGER     - Manager relationship (functional: max 1 current)
WORKS_ON        - Project involvement
WORKS_AT        - Employment (functional: max 1 current)
COLLEAGUE       - Work colleague connections
```

#### Preferences & Attributes
```
LIKES           - Positive preferences
DISLIKES        - Negative preferences
IS_ALLERGIC_TO  - Allergy information
PREFERS         - General preferences
HAS_INTEREST    - Interest areas
```

#### Causal & Logical
```
CAUSED_BY       - Causal relationship
BLOCKED_BY      - Blocking dependency
RESULTS_IN      - Consequence relationship
CONTRADICTS     - Contradiction marker
```

#### Meta Relationships
```
DERIVED_FROM    - Source tracking
SYNTHESIZED_FROM - Insight sources
SUPERSEDES      - Replacement relationship
```

## Functional Constraints

Certain edge types are "functional," meaning only one "current" instance can exist at a time:

```go
var FunctionalEdges = map[EdgeType]bool{
    EdgeTypeHasManager: true,  // One manager at a time
    EdgeTypePartnerIs:  true,  // One partner at a time
    EdgeTypeWorksAt:    true,  // One employer at a time
}
```

When a new functional edge is created, the existing one is automatically archived.

### Self-Curation Example

```
January: User says "My manager is Bob"
  → Creates: (User) -[HAS_MANAGER {status: current}]-> (Bob)

June: User says "My new manager is Alice"
  → Detects functional constraint violation
  → Archives: (User) -[HAS_MANAGER {status: archived}]-> (Bob)
  → Creates: (User) -[HAS_MANAGER {status: current}]-> (Alice)
  → Creates: (Alice) -[SUPERSEDES]-> (Bob)
```

## Activation System

Every node and edge has dynamic activation scores that determine relevance.

### Node Activation Properties

```go
type Node struct {
    UID          string    // Unique identifier
    Activation   float64   // 0.0 - 1.0 relevance score
    AccessCount  int64     // Number of times accessed
    LastAccessed time.Time // Last access timestamp
    Confidence   float64   // Extraction confidence
}
```

### Activation Algorithm

```
On Access:
    newActivation = min(activation + boostPerAccess, maxActivation)
    accessCount++
    lastAccessed = now

On Decay (hourly):
    daysSinceAccess = (now - lastAccessed) / 24h
    if daysSinceAccess > 1:
        decayFactor = (1 - decayRate) ^ daysSinceAccess
        newActivation = max(activation * decayFactor, minActivation)
```

### Default Configuration

```go
ActivationConfig{
    DecayRate:             0.05,  // 5% decay per day
    BoostPerAccess:        0.1,   // 10% boost per access
    MinActivation:         0.01,  // 1% minimum (pruning candidate)
    MaxActivation:         1.0,   // 100% maximum
    CoreIdentityThreshold: 0.8,   // 80% = core identity
}
```

## DGraph Schema Definition

```graphql
# Node types
type User {
    name
    description
    activation
    access_count
    last_accessed
}

type Entity {
    name
    description
    entity_type
    activation
    access_count
}

type Insight {
    name
    summary
    insight_type
    action_suggestion
    source_nodes
    confidence
}

type Pattern {
    name
    pattern_type
    frequency
    confidence_score
    predicted_action
    trigger_nodes
}

# Predicates with indexes
name: string @index(exact, term, fulltext) .
description: string @index(fulltext) .
activation: float @index(float) .
access_count: int @index(int) .
last_accessed: datetime @index(hour) .
created_at: datetime @index(hour) .
status: string @index(exact) .

# Relationship predicates
partner_is: uid @reverse .
has_manager: uid @reverse .
works_on: [uid] @reverse .
likes: [uid] @reverse .
is_allergic_to: [uid] @reverse .
synthesized_from: [uid] @reverse .
supersedes: uid @reverse .
```

## Query Examples

### Find High-Priority Memories

```graphql
{
  memories(func: ge(activation, 0.7), orderdesc: activation, first: 10) {
    uid
    name
    activation
    last_accessed
  }
}
```

### Find User's Graph (3 levels deep)

```graphql
{
  user(func: uid($userUID)) @recurse(depth: 3) {
    uid
    name
    partner_is
    has_manager
    works_on
    likes
    is_allergic_to
  }
}
```

### Find Contradictions

```graphql
{
  contradictions(func: has(has_manager)) @normalize {
    uid
    name
    edge_count: count(has_manager)
  }
}
```

### Search by Text

```graphql
{
  results(func: anyoftext(name, "project budget")) {
    uid
    name
    description
    activation
  }
}
```

## Graph Operations

### Creating a Node

```go
node := &graph.Node{
    Type:        graph.NodeTypeEntity,
    Name:        "Alex",
    Description: "User's partner",
    Activation:  0.5,
    Confidence:  0.9,
}
uid, err := graphClient.CreateNode(ctx, node)
```

### Creating an Edge

```go
err := graphClient.CreateEdge(ctx, 
    fromUID,           // Source node
    toUID,             // Target node
    graph.EdgeTypeLikes,  // Relationship type
    graph.EdgeStatusCurrent,  // Status
)
```

### Updating Activation

```go
// Boost on access
err := graphClient.IncrementAccessCount(ctx, uid, activationConfig)

// Direct update
err := graphClient.UpdateNodeActivation(ctx, uid, 0.95)
```
