# Reflective Memory Kernel - Complete Platform Documentation

> Enterprise AI Agent Platform with Persistent, Reflective Memory

---

# Product Overview

## Executive Summary

The **Reflective Memory Kernel (RMK)** is a transformative AI memory architecture that moves from reactive RAG (Retrieval-Augmented Generation) to proactive **Agent-Augmented Generation (AAG)**. Unlike traditional AI systems that start fresh with every conversation, RMK creates a persistent "digital subconscious" that learns, remembers, and anticipates.

This platform is designed for individuals and teams who need an AI that truly understands context over time—not just retrieving static facts, but actively maintaining, curating, and synthesizing knowledge.

---

## Vision & Mission

### Vision
To create AI systems that develop genuine long-term memory—systems that learn and grow with their users, providing increasingly personalized and proactive assistance over time.

### Mission
Deliver an enterprise-grade memory architecture that:
- **Remembers** user preferences, relationships, and context
- **Learns** patterns and anticipates needs
- **Reflects** on stored knowledge to discover insights
- **Collaborates** across teams with shared memory spaces

---

## The Problem We Solve

### Traditional AI Limitations

| Problem | Traditional AI | Reflective Memory Kernel |
|---------|---------------|--------------------------|
| **Amnesia** | Forgets everything after each session | Persistent memory across all interactions |
| **Static Knowledge** | Only retrieves what was stored | Actively synthesizes new insights |
| **No Personalization** | Same experience for everyone | Learns individual preferences and patterns |
| **Reactive Only** | Waits for user questions | Proactively surfaces relevant information |
| **Isolated** | Single-user silos | Team collaboration with shared context |
| **High Cost** | Every query hits expensive LLMs | 90% cost reduction via semantic caching |

---

## Core Concepts

### The Three-Phase Memory Loop

The Reflective Memory Kernel operates on a continuous three-phase loop:

```
┌─────────────────────────────────────────────────────────────────┐
│              PHASE 1: INGESTION                                 │
│  • Receives transcripts from conversations                      │
│  • Extracts entities, relationships, and facts via AI           │
│  • Writes structured knowledge to the Graph Database            │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              PHASE 2: REFLECTION (Async Rumination)             │
│  • Active Synthesis: Discovers emergent insights                │
│  • Predictive Anticipation: Detects behavioral patterns         │
│  • Self-Curation: Resolves contradictions automatically         │
│  • Dynamic Prioritization: Activation boost/decay               │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              PHASE 3: CONSULTATION                              │
│  • Synthesizes pre-computed insights                            │
│  • Returns coherent briefs, not raw facts                       │
│  • Provides proactive alerts and recommendations                │
└─────────────────────────────────────────────────────────────────┘
```

### Biological Memory Model

Unlike vector databases that treat all information equally, RMK implements a biological memory model:

- **Activation Decay**: Unused memories fade over time (configurable rate)
- **Reinforcement Boost**: Accessed memories get stronger (activation → 1.0)
- **Dynamic Reordering**: High-activation memories surface first in retrieval
- **Contradiction Resolution**: System automatically archives outdated facts

**Example:**
- January: User says "My manager is Bob" → Stored
- June: User says "My manager is Alice" → Bob archived, Alice active
- Query: "Who's my manager?" → Returns Alice (not both)

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Layer                                │
│     Web UI  •  Chat Interface  •  API  •  Integrations          │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│              Front-End Agent ("The Consciousness")               │
│  • Low-latency conversational interface                          │
│  • Consults Memory Kernel for context                            │
│  • Streams transcripts asynchronously                            │
└──────────────┬────────────────────────────────┬─────────────────┘
               │                                │
    ┌──────────▼──────────┐         ┌──────────▼──────────┐
    │    NATS JetStream   │         │   AI Services       │
    │  (Event Streaming)  │         │   (Python/FastAPI)  │
    └──────────┬──────────┘         └─────────────────────┘
               │
┌──────────────▼──────────────────────────────────────────────────┐
│              Memory Kernel ("The Subconscious")                  │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │    Pre-Cortex   │  │   Reflection    │  │  Consultation   │  │
│  │  (Cost Reducer) │  │    Engine       │  │    Handler      │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                        Data Layer                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │   DGraph     │  │    Redis     │  │      Qdrant          │   │
│  │ (Knowledge   │  │   (Cache &   │  │  (Vector Search)     │   │
│  │   Graph)     │  │   Sessions)  │  │                      │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Core Services

| Service | Technology | Purpose |
|---------|------------|---------|
| **Frontend Agent** | Go + Gorilla Mux | HTTP/WebSocket gateway, session management |
| **Memory Kernel** | Go | Graph operations, reflection, consultation |
| **AI Services** | Python + FastAPI | Entity extraction, synthesis, LLM routing |
| **DGraph** | Graph Database | Knowledge graph storage (nodes + edges) |
| **Redis** | Cache | Session storage, semantic cache |
| **Qdrant** | Vector DB | Semantic similarity search |
| **NATS JetStream** | Message Queue | Event streaming for async processing |

---

## Key Features

### 1. Self-Curation (Contradiction Resolution)

The system automatically resolves conflicting information:

```
Stored:     "My manager is Bob" (January 2025)
New Input:  "My manager is Alice" (June 2025)
Result:     Bob archived → Alice active as current manager
```

### 2. Active Synthesis (Insight Discovery)

Discovers hidden connections between stored facts:

```
Fact 1: "Alex loves Thai food"
Fact 2: "I have a peanut allergy"
Insight: "Thai food may contain peanuts - be careful when dining with Alex"
```

### 3. Predictive Anticipation (Pattern Detection)

Learns behavioral patterns for proactive assistance:

```
Pattern:    Every Monday user discusses "Project Alpha" with negative sentiment
Prediction: On Monday morning, prepare Project Alpha status brief
Action:     Proactively surface relevant context before meeting
```

### 4. Dynamic Prioritization (Memory Relevance)

Ensures most relevant memories surface first:

```
High Frequency Topic → Boosted activation → Appears first in retrieval
Stale/Unused Memory  → Decays over time  → Lower priority
Core Identity Traits → Protected from decay → Always accessible
```

### 5. Pre-Cortex (Semantic Caching)

A "cognitive firewall" that intercepts queries before hitting expensive LLMs:

```
Step 1: Check semantic cache for similar previous queries
Step 2: Classify intent (greeting, navigation, fact retrieval, complex)
Step 3: Handle simple requests without LLM (deterministic responses)
Step 4: Only complex queries reach the LLM

Result: 60-90% cost reduction on LLM API calls
```

### 6. Hybrid RAG Retrieval

Combines multiple retrieval methods for 100% recall:

| Method | Purpose | Contribution |
|--------|---------|--------------|
| **Vector Search** | Semantic similarity | Catches conceptually related memories |
| **Graph Traversal** | Relationship chains | Follows entity connections |
| **Activation Ranking** | Importance weighting | Surfaces high-value memories |
| **Temporal Filtering** | Recency bias | Prioritizes fresh information |

### 7. Workspace Collaboration

Google Docs-like sharing for AI memory spaces:

- **Role-Based Access**: Owner → Admin → Member permissions
- **Invitation System**: Invite by username or share link
- **Namespace Isolation**: Complete data separation between workspaces
- **Shared Context**: Teams can query collective memory

### 8. Multi-Provider LLM Routing

Intelligent routing across multiple LLM providers:

| Provider | Use Case | Priority |
|----------|----------|----------|
| NVIDIA NIM | High-speed inference | Primary |
| OpenAI | GPT-4 for complex tasks | Secondary |
| Anthropic | Claude for nuanced responses | Secondary |
| Ollama | Local development | Fallback |

---

## Technical Specifications

### Data Model

**Node Types:**
- `User`: Platform users
- `Entity`: People, places, concepts, organizations
- `Fact`: Stored knowledge statements
- `Insight`: AI-generated connections
- `Pattern`: Behavioral patterns
- `Conversation`: Chat histories

**Edge Types:**
- `knows_person`, `works_at`, `likes`, `dislikes`
- `has_preference`, `created_by`, `related_to`
- Functional relationships (e.g., "has_manager" - only one active)

**Edge Metadata (Facets):**
- `activation`: 0.0 - 1.0 (memory accessibility)
- `confidence`: Truthfulness score
- `status`: current / archived / pending
- `created_at`, `updated_at`: Timestamps

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `DECAY_RATE` | 0.05 | Activation decay per reflection cycle |
| `REFLECTION_INTERVAL` | 5 min | Time between reflection cycles |
| `MIN_REFLECTION_BATCH` | 10 | Minimum nodes per reflection |
| `MAX_REFLECTION_BATCH` | 100 | Maximum nodes per reflection |
| `CACHE_SIMILARITY` | 0.95 | Semantic cache hit threshold |

### Performance Targets

| Metric | Target |
|--------|--------|
| Query Latency (P95) | < 500ms |
| Cache Hit Rate | > 60% |
| Uptime | 99.9% |
| Memory Accuracy | > 95% |
| LLM Cost Reduction | 60-90% |

---

## Competitive Advantages

### vs. Traditional RAG Systems

| Capability | Traditional RAG | Reflective Memory Kernel |
|------------|-----------------|--------------------------|
| Memory Persistence | ❌ Session-only | ✅ Permanent |
| Self-Curation | ❌ Manual | ✅ Automatic |
| Insight Discovery | ❌ No | ✅ Yes |
| Pattern Detection | ❌ No | ✅ Yes |
| Contradiction Handling | ❌ Returns both | ✅ Archives old |
| Cost Optimization | ❌ Every query hits LLM | ✅ 90% cached |
| Team Collaboration | ❌ Single-user | ✅ Multi-workspace |

### vs. Vector-Only Systems

| Capability | Vector DB Only | Reflective Memory Kernel |
|------------|---------------|--------------------------|
| Relationship Queries | ❌ Poor | ✅ Excellent (Graph) |
| Recall Rate | ~70% | ~100% (Hybrid) |
| Temporal Awareness | ❌ No decay | ✅ Biological model |
| Logical Reasoning | ❌ Limited | ✅ Graph traversal |

---

## Use Cases

### Individual Users
- Personal AI that remembers preferences, relationships, and history
- Long-term project context across months of work
- Automated note organization and insight discovery

### Teams
- Shared customer context for support teams
- Collective knowledge base for research groups
- Organizational memory for onboarding new team members

### Enterprise
- Customer relationship memory at scale
- Compliance-ready audit trails
- Secure multi-tenant deployment

---

## Pricing Tiers

| Tier | Price | Memories | Workspaces | Features |
|------|-------|----------|------------|----------|
| **Free/Trial** | $0 | 500 | 0 | Basic chat, limited graph |
| **Pro** | $15/mo | 10,000 | 1 personal | Full dashboard, kernel ops, API |
| **Team** | $50/mo | 50,000 | 10 shared | Collaboration, share links |
| **Enterprise** | Custom | Unlimited | Unlimited | SSO, audit, dedicated infra |

---

## Global Design Elements

### Color Scheme
- **Primary**: Deep Purple (#7C3AED) - Represents intelligence and memory
- **Secondary**: Cyan (#06B6D4) - Represents clarity and neural connections
- **Accent**: Amber (#F59E0B) - Represents insights and highlights
- **Alert**: Red (#EF4444) - For urgent notifications
- **Success**: Green (#10B981) - For confirmations and positive states
- **Background**: Deep Black (#09090B) - Modern dark theme foundation

### Typography
- **Headings**: Inter (Sans-serif) - Clean, modern appearance
- **Body**: Inter (Sans-serif) - Optimal readability
- **Data Values**: JetBrains Mono (Monospace) - Clear number display

### Layout Principles
- Responsive design with minimum 1200px optimization for dashboards
- Card-based UI for easy scanning of information
- Consistent spacing (8px grid system)
- Clear visual hierarchy with section dividers
- Glassmorphism effects for depth and modern aesthetics

---

# Super Admin Interface

## 1. Dashboard (Home)

The main landing page displaying critical business metrics, system health, and urgent action items requiring attention. Includes real-time data visualizations and quick-action cards.

### Key Metrics Displayed
- **Total Users**: Complete count of registered platform users
- **Total Admins**: Number of users with administrative privileges
- **Active Sessions**: Real-time user activity monitoring
- **System Uptime**: Platform availability percentage
- **Memory Kernel Health**: Graph database and AI service status

### Quick Actions
- Trigger Reflection Cycle
- View Recent Activity
- Access User Management
- System Health Check

---

## 2. User Management

### All Users
A comprehensive user directory with advanced filtering options. Display user profiles with subscription status, sign-up date, referral source, and activity metrics. Include actions for account management, impersonation for troubleshooting, and direct communication.

**Features:**
- User list with avatar and role badges
- Search and filter functionality
- Bulk selection for batch actions
- Sortable columns (username, role, created date, last active)

### Trial Users
Focused view of users in trial period with countdown timers to expiration. Show engagement metrics, feature usage, and conversion probability scores. Include batch actions for sending targeted communications and extending trials for promising prospects.

**Trial Metrics:**
- Days remaining in trial
- Feature activation percentage
- Memory nodes created
- Queries made
- Conversion probability score

### Paid Subscribers
Complete management of paying customers with subscription details, renewal dates, and lifetime value calculations. Provide tools for plan changes, subscription pausing, and special offer management to increase retention.

**Subscription Details:**
- Plan tier (Pro/Team/Enterprise)
- Billing cycle and amount
- Renewal date
- Lifetime value (LTV)
- Churn risk indicator

### Workspace Members
Directory of all workspace members in the system with relationship mapping to primary users. Display verification status, role demographics, and engagement levels. Include tools for targeted communications.

**Member Information:**
- Parent workspace association
- Role (Admin/Subuser)
- Join date
- Invitation source
- Activity level

---

## 3. Team Management

### Support Team
Performance dashboard for support agents with ticket resolution metrics, quality scores, and workload distribution. Provide tools for reassigning tickets, adjusting agent availability, and reviewing customer satisfaction ratings.

**Support Metrics:**
- Tickets resolved (daily/weekly/monthly)
- Average resolution time
- Customer satisfaction score (CSAT)
- First response time
- Escalation rate

### Operations Team
Oversight of operations activities including campaign management, system optimization, and process efficiency metrics. Include approval workflows for major operational changes and resource allocation tools.

**Operations Metrics:**
- System performance indicators
- Campaign effectiveness
- Resource utilization
- Process efficiency scores

### Affiliate Program
Comprehensive management of affiliate relationships, including application approvals, commission structure adjustments, and performance monitoring. Provide tools for managing referral codes, reviewing marketing materials, and resolving payment disputes.

**Affiliate Management:**
- Affiliate applications queue
- Active affiliates list
- Commission rate management
- Referral code generation
- Performance tracking

---

## 4. Finance

### Revenue Reports
Detailed financial analytics with revenue breakdowns by plan type, user cohort, and acquisition channel. Include forecasting tools, churn analysis, and lifetime value projections with exportable reports for accounting.

**Revenue Breakdown:**
- Monthly Recurring Revenue (MRR)
- Annual Recurring Revenue (ARR)
- Revenue by plan tier
- Revenue by acquisition channel
- Growth rate trends

### Commission Payouts
Management interface for affiliate commission calculations, payment scheduling, and transaction history. Include verification workflows for large payouts, tax documentation management, and payment method administration.

**Payout Features:**
- Pending payouts queue
- Approved payouts history
- Payment method verification
- Tax document collection
- Dispute management

### Pricing Management
Tools for adjusting subscription pricing, creating promotional offers, and A/B testing different price points. Include impact analysis for proposed changes and scheduled implementation of pricing updates.

**Pricing Tools:**
- Plan pricing editor
- Promotional code creator
- Discount management
- Price change scheduling
- Impact simulation

---

## 5. System

### Access Controls
Comprehensive user role and permission management for all system users (admin team, operations, support, affiliates). Include audit logs of permission changes, session management, and security policy enforcement.

**Permission Levels:**
- Super Admin: Full system access
- Admin: User and system management
- Operations: Campaign and analytics access
- Support: User assistance tools only
- Affiliate: Self-service marketing tools

### Feature Management
Interface for enabling/disabling platform features, rolling out updates, and managing feature flags for testing. Include A/B testing tools and usage analytics for feature adoption.

**Feature Flags:**
- Pre-Cortex Caching (enable/disable)
- Reflection Engine (frequency adjustment)
- Vision Processing (toggle)
- Workspace Collaboration (enable/disable)
- API Access (rate limit configuration)

### Maintenance
System maintenance scheduling, backup management, and performance optimization tools. Include notification management for planned downtime and emergency maintenance procedures.

**Maintenance Tools:**
- Backup scheduling
- Database optimization triggers
- Cache clearing
- Service restart controls
- Downtime notification scheduler

---

## 6. Memory Kernel Controls

### Reflection Cycle Management
Direct controls for the Memory Kernel's reflection engine, allowing manual triggering and configuration of automated reflection cycles.

**Reflection Modules:**
- **Synthesis**: Discovers hidden connections between entities
- **Anticipation**: Detects behavioral patterns for proactive alerts
- **Curation**: Resolves contradictions in stored memories
- **Prioritization**: Manages activation decay and boost

**Configuration Options:**
- Reflection interval (default: 5 minutes)
- Minimum batch size (default: 10)
- Maximum batch size (default: 100)
- Activation decay rate (default: 0.05)

### Graph Database Operations
Direct access to DGraph knowledge graph operations for advanced troubleshooting and data management.

**Available Operations:**
- Schema inspection
- Node count queries
- Edge density analysis
- Namespace statistics
- Data export/import

### Pre-Cortex Statistics
Monitoring dashboard for the semantic caching layer that reduces LLM costs.

**Pre-Cortex Metrics:**
- Total requests processed
- Cache hit rate (target: >60%)
- Reflex responses (handled without LLM)
- LLM passthrough rate
- Estimated cost savings

---

## 7. Emergency Access

### Pending Requests
Queue of active emergency access requests with verification status, request details, and countdown timers for SLA compliance. Include verification tools, communication templates, and approval workflows.

**Request Details:**
- Requester information
- Verification status
- Request timestamp
- SLA countdown
- Priority level

### Access Logs
Comprehensive audit trail of all emergency access grants, including requestor information, verification methods used, data accessed, and approving administrator. Include filtering and export capabilities for compliance reporting.

**Log Information:**
- Access timestamp
- Approver identity
- Access scope
- Duration granted
- Actions performed

### Protocol Settings
Configuration interface for emergency access rules, verification requirements, and approval workflows. Include customization of notification settings, documentation requirements, and escalation procedures.

---

## 8. Reports

### Executive Summary
High-level business performance reports designed for stakeholder presentations. Include key metrics, growth trends, and strategic insights with exportable formats and presentation-ready visualizations.

**Summary Metrics:**
- User growth rate
- Revenue trends
- Churn analysis
- Feature adoption
- Platform health score

### User Analytics
In-depth analysis of user behavior, engagement patterns, and feature adoption. Include cohort analysis, retention metrics, and predictive models for churn prevention.

**Analytics Views:**
- User journey mapping
- Feature usage heatmaps
- Retention curves
- Engagement scoring
- Churn prediction

### Team Performance
Comparative analysis of team efficiency metrics across support, operations, and affiliate channels. Include goal tracking, historical trends, and resource utilization insights.

---

## 9. Activity Log

Real-time feed of all administrative actions performed on the platform. Provides complete audit trail for security and compliance.

**Log Entry Format:**
```
Timestamp | User ID | Action | Details
2026-01-03 10:00:00 | admin_user | UPDATE_ROLE | Changed 'alice' role to 'admin'
2026-01-03 09:45:00 | admin_user | DELETE_USER | Removed user 'spam_account'
2026-01-03 09:30:00 | admin_user | TRIGGER_REFLECTION | Manual reflection cycle initiated
```

**Filterable By:**
- Date range
- User ID
- Action type
- Severity level

---

## 10. Settings

### Personal Account
- Profile information management
- Password change
- Two-factor authentication setup
- Session management

### Notification Preferences
- Email notification toggles
- Push notification settings
- Alert thresholds
- Digest frequency

### Interface Customization
- Theme selection (Dark/Light/System)
- Dashboard widget arrangement
- Default view preferences
- Timezone settings

---

# Operations Interface

## 1. Dashboard (Home)

Operational overview showing user acquisition metrics, campaign performance, and affiliate activity. Include task prioritization widgets and goal tracking visualizations.

### Key Operational Metrics
- Daily active users (DAU)
- Weekly active users (WAU)
- User acquisition by channel
- Campaign conversion rates
- Affiliate referral volume

---

## 2. User Lifecycle

### Onboarding Funnel
Visual representation of user journey from sign-up to activation with conversion rates at each step.

**Funnel Stages:**
1. Registration (100%)
2. Email verification (85%)
3. First conversation (70%)
4. Memory created (55%)
5. Return visit (40%)
6. Paid conversion (15%)

### Engagement Tracking
Tools for monitoring user engagement levels and identifying at-risk accounts.

**Engagement Indicators:**
- Login frequency
- Session duration
- Features used
- Queries per session
- Memory growth rate

### Conversion Optimization
A/B testing tools and optimization strategies for improving trial-to-paid conversion rates.

---

## 3. Campaigns

### Email Campaigns
Management of automated and manual email campaigns for user engagement, re-activation, and promotions.

**Campaign Types:**
- Welcome series
- Trial expiration reminders
- Feature announcements
- Re-engagement sequences
- Upgrade promotions

### In-App Messaging
Configuration of contextual messages, tooltips, and promotional banners within the application.

### Push Notifications
Management of browser and mobile push notifications for time-sensitive communications.

---

## 4. Affiliate Management

### Applications
Review queue for new affiliate applications with approval/rejection workflows.

### Performance Tracking
Dashboard showing affiliate performance metrics and commission calculations.

### Marketing Resources
Library of approved marketing materials for affiliate distribution.

---

## 5. Reports

### Acquisition Analytics
Detailed breakdown of user acquisition sources and channel effectiveness.

### Conversion Reports
Analysis of conversion rates across different user segments and time periods.

### Retention Analysis
Cohort-based retention analysis with identification of factors affecting user retention.

---

# Trial User Panel

## 1. Dashboard (Home)

Overview of trial features, usage limits, and days remaining in trial period. Clear upgrade prompts and feature discovery guidance.

### Trial Status Display
- Days remaining counter
- Features unlocked/locked indicator
- Usage limits visualization
- Upgrade call-to-action

---

## 2. Memory Graph (Limited)

Visualize stored memories with a cap of 500 nodes during trial period.

**Available Features:**
- 2D graph visualization
- Basic node inspection
- Search functionality

**Locked Features:**
- Kernel operations
- Community detection
- Temporal decay controls
- Spreading activation

---

## 3. Chat Interface

Conversational AI interface with basic memory capabilities.

**Available:**
- Up to 50 queries per day
- Personal namespace only
- Basic memory recall

**Locked:**
- Group/workspace context
- Document context
- Unlimited queries

---

## 4. Settings (Basic)

Limited settings access for trial users.

**Available:**
- Profile management
- Notification preferences
- Theme selection

**Locked:**
- API key generation
- Data export
- Advanced privacy settings

---

## 5. Upgrade Prompts

Strategic placement of upgrade prompts throughout the trial experience.

**Trigger Points:**
- Reaching storage limits
- Attempting locked features
- Trial expiration approaching
- High engagement detection

---

# Paid User Panel (Pro Tier)

## 1. Dashboard (Home)

Full-featured dashboard with complete graph visualization and kernel operations.

### Statistics Bar
- **Indexed Nodes**: Total memories stored
- **Edge Density**: Relationship connections
- **Memory Usage**: Storage consumption
- **System Status**: Real-time health

### Kernel Operations Panel

#### Spreading Activation
Activate neural-like spreading across the knowledge graph from a starting node.

**Controls:**
- Start node input
- Depth slider (1-5 hops)
- Decay factor adjustment
- Execute button

#### Community Detection
Identify clusters of related memories using the Leiden algorithm.

**Output:**
- Number of communities found
- Community size distribution
- Modularity score

#### Temporal Decay
Apply time-based forgetting to reduce noise from outdated memories.

**Controls:**
- Lookback period slider (7-90 days)
- Decay rate adjustment
- Preview affected nodes
- Apply button

---

## 2. Chat Interface (Full)

Complete conversational AI with unlimited queries and full memory context.

**Features:**
- Unlimited daily queries
- Full memory context injection
- Conversation history
- Namespace switching (personal/group)
- Document context integration

---

## 3. Document Ingestion

Full document processing capabilities for building knowledge from files.

**Supported Formats:**
- PDF (with vision extraction for charts)
- TXT
- DOCX
- JSON
- Markdown

**Processing Pipeline:**
1. Document upload
2. Text extraction
3. Entity extraction (AI-powered)
4. Relationship inference
5. Knowledge graph integration
6. Vector embedding storage

---

## 4. Settings (Full)

### Profile
- Username and email management
- Avatar upload
- Bio and preferences

### Notifications
- Email notifications toggle
- Push notifications toggle
- Insight alerts toggle
- Product updates toggle

### API Keys
- Generate production API keys
- View usage statistics
- Revoke keys
- Rate limit visibility

### Appearance
- Dark mode
- Light mode
- System preference

### Privacy & Security
- Two-factor authentication
- Password change
- Session management
- Login history

### Data Management
- Export all data
- Delete specific memories
- Account deletion

---

# Paid User Panel (Team Tier)

All Pro features plus:

## 1. Groups/Workspaces

Full workspace collaboration capabilities.

### Workspace Management
- Create new workspaces
- Edit workspace details
- Delete workspaces
- View all member workspaces

### Member Management
- Invite by username
- Generate share links
- Set link expiration
- Set usage limits
- Remove members
- Transfer ownership

### Roles
| Role | Permissions |
|------|-------------|
| Owner | Full control, cannot be removed |
| Admin | Invite/remove members, settings |
| Member | Read/write memories |

### Share Links
Generate shareable links for quick team onboarding.

**Link Options:**
- Maximum uses (1-100 or unlimited)
- Expiration time (hours/days/never)
- Default role for joiners
- Revocation capability

---

## 2. Team Analytics

Dashboard showing workspace-wide activity and member contributions.

**Metrics:**
- Active members
- Memory contributions by member
- Query volume by member
- Storage usage by workspace

---

# Support Interface

## 1. Dashboard (Home)

Support-focused overview showing ticket queue, user assistance requests, and key support metrics.

### Support Metrics
- Open tickets
- Average response time
- Resolution rate
- Customer satisfaction score

---

## 2. User Lookup

Quick access to user account information for troubleshooting.

### Lookup Capabilities
- Search by username
- Search by email
- View subscription status
- View recent activity
- View memory statistics

### Actions Available
- Reset password (send link)
- Extend trial period
- Apply promotional credit
- Escalate to admin

---

## 3. Ticket Management

### Active Tickets
Queue of open support requests with priority sorting.

**Ticket Information:**
- User details
- Issue category
- Priority level
- Assigned agent
- SLA countdown

### Ticket Actions
- Respond to user
- Change priority
- Reassign agent
- Escalate to admin
- Mark resolved

---

## 4. Quick Actions

Library of one-click support actions for common requests.

**Available Actions:**
- Password reset email
- Trial extension (7 days)
- Verification email resend
- Export user data
- Clear user cache

---

## 5. Knowledge Base

### Common Solutions
Repository of verified solutions to frequent issues.

**Categories:**
- Account access issues
- Subscription problems
- Feature questions
- Technical troubleshooting
- Billing inquiries

### Template Library
Pre-written responses for common scenarios.

### Feature Guides
Documentation of platform features for reference during support.

---

## 6. Reports

### My Performance
Personal productivity metrics for support agents.

**Metrics:**
- Tickets resolved
- Average handle time
- Customer satisfaction
- Quality score

### Team Metrics
Collaborative performance dashboard.

### User Satisfaction
Analysis of customer feedback and satisfaction trends.

---

# API Reference

## Admin Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/admin/users` | List all users |
| `GET` | `/api/admin/users/{username}` | Get user details |
| `PUT` | `/api/admin/users/{username}/role` | Update user role |
| `DELETE` | `/api/admin/users/{username}` | Delete user |
| `GET` | `/api/admin/system/stats` | System statistics |
| `POST` | `/api/admin/system/reflection` | Trigger reflection |
| `GET` | `/api/admin/groups` | List all groups |
| `DELETE` | `/api/admin/groups/{id}` | Delete group |
| `GET` | `/api/admin/activity` | Activity log |

## User Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/chat` | Send chat message |
| `GET` | `/api/conversations` | List conversations |
| `POST` | `/api/upload` | Upload document |
| `GET` | `/api/groups` | List user's groups |
| `POST` | `/api/groups` | Create group |
| `GET` | `/api/stats` | User memory stats |
| `POST` | `/api/consult` | Query memory kernel |
| `GET` | `/api/graph` | Get graph visualization data |

## Workspace Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/workspaces/{id}/invite` | Invite user |
| `POST` | `/api/workspaces/{id}/share-link` | Create share link |
| `GET` | `/api/workspaces/{id}/members` | List members |
| `DELETE` | `/api/workspaces/{id}/members/{user}` | Remove member |
| `POST` | `/api/join/{token}` | Join via share link |
| `POST` | `/api/invitations/{id}/accept` | Accept invitation |
| `POST` | `/api/invitations/{id}/decline` | Decline invitation |

---

# Security Model

## Authentication
- JWT-based authentication
- Token expiration: 24 hours
- Refresh token support
- Secure password hashing (bcrypt)

## Authorization
- Role-based access control (RBAC)
- Namespace isolation for user data
- Admin routes protected by middleware
- API rate limiting

## Data Protection
- Namespace-scoped queries prevent data leakage
- Strict filtering: `@filter(eq(namespace, $current_namespace))`
- Audit logging for all admin actions
- Encrypted data at rest

---

# Technical Architecture

## Core Services

| Service | Port | Description |
|---------|------|-------------|
| Frontend Agent | 3000 | HTTP/WebSocket gateway |
| Memory Kernel | 9000 | Graph operations & reflection |
| AI Services | 8000 | LLM routing & extraction |
| DGraph | 9080 | Knowledge graph database |
| Redis | 6379 | Caching & session storage |
| NATS | 4222 | Event streaming |
| Qdrant | 6333 | Vector similarity search |

## Data Flow

```
User Request → Frontend Agent → Memory Kernel → DGraph
                    ↓
              AI Services → LLM Providers (NVIDIA/OpenAI/Anthropic)
                    ↓
              Pre-Cortex (Semantic Cache) → Response
```

---

# File Reference

| Component | Frontend | Backend |
|-----------|----------|---------|
| Admin Panel | `pages/Admin.tsx` | `agent/admin_handlers.go` |
| Dashboard | `pages/Dashboard.tsx` | `agent/server.go` |
| Chat | `pages/Chat.tsx` | `agent/server.go` |
| Settings | `pages/Settings.tsx` | `agent/server.go` |
| Groups | `pages/Groups.tsx` | `kernel/kernel.go` |
| Auth | `pages/Auth.tsx` | `agent/server.go` |
| Memory Kernel | - | `kernel/` directory |
| Reflection | - | `reflection/` directory |
| Pre-Cortex | - | `precortex/` directory |
| Graph Client | - | `graph/client.go` |

---

*Document Version: 1.0*
*Last Updated: January 3, 2026*
*Platform: Reflective Memory Kernel*
