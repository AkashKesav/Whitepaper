# Admin Panel Documentation

**Version:** Updated UI-Aligned Specification  
**Last Updated:** January 3, 2026

---

## Purpose

This document provides a **UI-first, sidebar-driven admin specification** for the Reflective Memory Kernel platform. It follows the layout, tone, hierarchy, and presentation style of modern admin panel design documentation.

---

## Global Design Elements

### Color Scheme
- **Primary**: Deep Purple (#7C3AED) - Represents intelligence and authority
- **Secondary**: Cyan (#06B6D4) - Represents clarity and efficiency
- **Accent**: Amber (#F59E0B) - Represents value and importance
- **Alert**: Red (#EF4444) - For urgent notifications
- **Success**: Green (#10B981) - For confirmations
- **Background**: Deep Black (#09090B) - Modern dark theme foundation

### Typography
- **Headings**: Inter (Sans-serif) - Clean, modern appearance
- **Body**: Inter (Sans-serif) - Optimal readability
- **Data Values**: JetBrains Mono (Monospace) - Clear number display

### Layout Principles
- Responsive design with minimum 1200px optimization for admin dashboards
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
- **All Users**
- **Trial Users**
- **Paid Subscribers**
- **Workspace Members**

---

### All Users

A comprehensive user directory with advanced filtering options. Display user profiles with subscription status, sign-up date, referral source, and activity metrics. Include actions for account management, impersonation for troubleshooting, and direct communication.

**Display Fields:**
| Field | Description |
|-------|-------------|
| Username | Primary identifier with avatar |
| Role | admin / user badge |
| Status | active / suspended / pending |
| Subscription | trial / pro / team / enterprise |
| Joined | Registration timestamp |
| Last Active | Most recent activity |
| Memory Count | Total nodes stored |

**Available Actions:**
- View detailed profile
- Impersonate for troubleshooting
- Edit subscription
- Reset password
- Delete account

---

### Trial Users

Focused view of users in trial period with countdown timers to expiration. Show engagement metrics, feature usage, and conversion probability scores. Include batch actions for sending targeted communications and extending trials for promising prospects.

**Trial-Specific Metrics:**
| Metric | Description |
|--------|-------------|
| Days Remaining | Countdown to trial expiration |
| Activation Rate | % of key features used |
| Memory Nodes Created | Content engagement |
| Queries Made | Conversation activity |
| Login Frequency | Engagement pattern |
| Conversion Score | AI-predicted conversion probability |

**Batch Actions:**
- Extend trial (7/14/30 days)
- Send targeted email
- Apply promotional offer
- Export segment

---

### Paid Subscribers

Complete management of paying customers with subscription details, renewal dates, and lifetime value calculations. Provide tools for plan changes, subscription pausing, and special offer management to increase retention.

**Subscription Details:**
| Field | Description |
|-------|-------------|
| Plan Tier | Pro / Team / Enterprise |
| Billing Cycle | Monthly / Annual |
| Amount | Current billing amount |
| Renewal Date | Next billing date |
| LTV | Calculated lifetime value |
| Churn Risk | Low / Medium / High indicator |
| Payment Method | Card / PayPal / Invoice |

**Management Actions:**
- Upgrade/downgrade plan
- Pause subscription
- Apply discount
- Issue refund
- Cancel subscription

---

### Workspace Members

Directory of all workspace members in the system with relationship mapping to primary users. Display verification status, role demographics, and engagement levels. Include tools for targeted communications.

**Member Information:**
| Field | Description |
|-------|-------------|
| Parent Workspace | Associated workspace name |
| Owner | Workspace owner username |
| Role | admin / subuser |
| Join Method | invite / share-link |
| Join Date | When they joined |
| Activity Level | Active / Inactive |

---

## 3. Team Management

- **Support Team**
- **Operations Team**
- **Affiliate Program**

---

### Support Team

Performance dashboard for support agents with ticket resolution metrics, quality scores, and workload distribution. Provide tools for reassigning tickets, adjusting agent availability, and reviewing customer satisfaction ratings.

**Performance Metrics:**
| Metric | Target | Description |
|--------|--------|-------------|
| Tickets Resolved | Track daily/weekly/monthly | Total closed tickets |
| Avg Resolution Time | < 4 hours | Time from open to close |
| CSAT Score | > 4.5/5 | Customer satisfaction rating |
| First Response Time | < 30 min | Initial reply speed |
| Escalation Rate | < 10% | Tickets requiring escalation |
| Quality Score | > 90% | Peer review rating |

**Management Tools:**
- View individual agent dashboard
- Reassign tickets
- Adjust availability status
- Review satisfaction ratings
- Set performance goals

---

### Operations Team

Oversight of operations activities including campaign management, system optimization, and process efficiency metrics. Include approval workflows for major operational changes and resource allocation tools.

**Operational Metrics:**
| Metric | Description |
|--------|-------------|
| Campaign Performance | Conversion rates by campaign |
| User Acquisition Cost | Average CAC by channel |
| Feature Adoption | % users using key features |
| System Efficiency | Resource utilization % |
| Process Completion | Workflow completion rates |

**Approval Workflows:**
- Campaign approvals
- Pricing changes
- Feature rollouts
- Budget allocations

---

### Affiliate Program

Comprehensive management of affiliate relationships, including application approvals, commission structure adjustments, and performance monitoring. Provide tools for managing referral codes, reviewing marketing materials, and resolving payment disputes.

**Affiliate Management:**
| Feature | Description |
|---------|-------------|
| Application Queue | Pending affiliate applications |
| Active Affiliates | Currently active partners |
| Commission Rates | Tiered commission structure |
| Referral Codes | Code generation and tracking |
| Performance Dashboard | Conversion and revenue metrics |
| Payout Management | Payment scheduling and disputes |

---

## 4. Finance

- **Revenue Reports**
- **Commission Payouts**
- **Pricing Management**

---

### Revenue Reports

Detailed financial analytics with revenue breakdowns by plan type, user cohort, and acquisition channel. Include forecasting tools, churn analysis, and lifetime value projections with exportable reports for accounting.

**Revenue Breakdown:**
| View | Description |
|------|-------------|
| MRR | Monthly recurring revenue |
| ARR | Annual recurring revenue |
| By Plan Tier | Revenue per subscription type |
| By Channel | Revenue by acquisition source |
| By Cohort | Revenue by sign-up month |
| Growth Rate | Month-over-month change |

**Analysis Tools:**
- Revenue forecasting
- Churn impact analysis
- LTV projections
- Export to CSV/PDF

---

### Commission Payouts

Management interface for affiliate commission calculations, payment scheduling, and transaction history. Include verification workflows for large payouts, tax documentation management, and payment method administration.

**Payout Dashboard:**
| Status | Description |
|--------|-------------|
| Pending | Awaiting approval |
| Approved | Ready for payment |
| Processing | Payment in progress |
| Completed | Successfully paid |
| Disputed | Under review |

**Verification Workflows:**
- Large payout review (>$1000)
- New affiliate first payment
- Unusual activity flag
- Tax document verification

---

### Pricing Management

Tools for adjusting subscription pricing, creating promotional offers, and A/B testing different price points. Include impact analysis for proposed changes and scheduled implementation of pricing updates.

**Pricing Tools:**
| Tool | Description |
|------|-------------|
| Plan Editor | Modify plan features and pricing |
| Promo Codes | Create and manage discount codes |
| A/B Testing | Test pricing variations |
| Impact Simulator | Forecast revenue impact |
| Scheduled Changes | Plan future price updates |

---

## 5. System

- **Access Controls**
- **Feature Management**
- **Maintenance**

---

### Access Controls

Comprehensive user role and permission management for all system users (admin team, operations, support, affiliates). Include audit logs of permission changes, session management, and security policy enforcement.

**Role Hierarchy:**
| Role | Access Level | Permissions |
|------|--------------|-------------|
| Super Admin | Full | All system access |
| Admin | High | User + system management |
| Operations | Medium | Campaigns + analytics |
| Support | Limited | User assistance only |
| Affiliate | Minimal | Self-service marketing |

**Security Features:**
- Role assignment/revocation
- Permission audit logs
- Session management
- IP restrictions
- 2FA enforcement

---

### Feature Management

Interface for enabling/disabling platform features, rolling out updates, and managing feature flags for testing. Include A/B testing tools and usage analytics for feature adoption.

**Feature Flags:**
| Feature | Status | Description |
|---------|--------|-------------|
| Pre-Cortex | Active | Semantic caching layer |
| Reflection Engine | Active | Background reflection |
| Vision Processing | Active | PDF chart extraction |
| Workspace Collaboration | Active | Team sharing |
| API Access | Active | External API |
| Beta Features | Configurable | Experimental features |

**Rollout Controls:**
- Enable/disable toggle
- User percentage rollout
- User segment targeting
- A/B test configuration

---

### Maintenance

System maintenance scheduling, backup management, and performance optimization tools. Include notification management for planned downtime and emergency maintenance procedures.

**Maintenance Tools:**
| Tool | Description |
|------|-------------|
| Backup Scheduler | Configure automatic backups |
| Database Optimization | Trigger DGraph optimization |
| Cache Management | Clear Redis caches |
| Service Controls | Restart individual services |
| Downtime Scheduler | Plan maintenance windows |
| Emergency Procedures | Quick-action runbooks |

---

## 6. Memory Kernel Controls

- **Reflection Cycle**
- **Graph Operations**
- **Pre-Cortex Stats**

---

### Reflection Cycle Management

Direct controls for the Memory Kernel's reflection engine, allowing manual triggering and configuration of automated reflection cycles.

**Reflection Modules:**
| Module | Function |
|--------|----------|
| Synthesis | Discovers hidden connections between entities |
| Anticipation | Detects behavioral patterns for proactive alerts |
| Curation | Resolves contradictions in stored memories |
| Prioritization | Manages activation decay and boost |

**Configuration:**
| Parameter | Default | Range |
|-----------|---------|-------|
| Reflection Interval | 5 min | 1-60 min |
| Minimum Batch | 10 | 5-50 |
| Maximum Batch | 100 | 50-500 |
| Decay Rate | 0.05 | 0.01-0.20 |

**Actions:**
- Trigger immediate reflection
- View reflection history
- Adjust parameters
- Monitor module performance

---

### Graph Database Operations

Direct access to DGraph knowledge graph operations for advanced troubleshooting and data management.

**Available Operations:**
| Operation | Description |
|-----------|-------------|
| Schema View | Current graph schema |
| Node Count | Total nodes by type |
| Edge Density | Relationship statistics |
| Namespace Stats | Per-user/workspace metrics |
| Query Console | Direct DGraph queries |
| Data Export | Full namespace export |

---

### Pre-Cortex Statistics

Monitoring dashboard for the semantic caching layer that reduces LLM costs.

**Pre-Cortex Metrics:**
| Metric | Target | Description |
|--------|--------|-------------|
| Total Requests | - | All processed requests |
| Cache Hit Rate | > 60% | Semantic cache matches |
| Reflex Responses | > 20% | Handled without LLM |
| LLM Passthrough | < 20% | Requires full LLM |
| Avg Latency (cached) | < 50ms | Cache response time |
| Avg Latency (LLM) | < 500ms | LLM response time |
| Cost Savings | - | Estimated $ saved |

---

## 7. Emergency Access

- **Pending Requests**
- **Access Logs**
- **Protocol Settings**

---

### Pending Requests

Queue of active emergency access requests with verification status, request details, and countdown timers for SLA compliance. Include verification tools, communication templates, and approval workflows.

**Request Queue Fields:**
| Field | Description |
|-------|-------------|
| Requester | User requesting access |
| Target Account | Account to access |
| Reason | Stated justification |
| Verification Status | Identity confirmation |
| SLA Countdown | Time remaining for response |
| Priority | Low / Medium / High / Critical |

**Actions:**
- Verify identity
- Approve access
- Deny with reason
- Escalate to supervisor

---

### Access Logs

Comprehensive audit trail of all emergency access grants, including requestor information, verification methods used, data accessed, and approving administrator. Include filtering and export capabilities for compliance reporting.

**Log Entry Fields:**
| Field | Description |
|-------|-------------|
| Timestamp | When access was granted |
| Approver | Admin who approved |
| Requester | Who requested access |
| Target Account | Account accessed |
| Access Scope | What data was visible |
| Duration | How long access lasted |
| Actions Taken | Operations performed |

---

### Protocol Settings

Configuration interface for emergency access rules, verification requirements, and approval workflows. Include customization of notification settings, documentation requirements, and escalation procedures.

---

## 8. Reports

- **Executive Summary**
- **User Analytics**
- **Team Performance**

---

### Executive Summary

High-level business performance reports designed for stakeholder presentations. Include key metrics, growth trends, and strategic insights with exportable formats and presentation-ready visualizations.

**Summary Dashboard:**
| Section | Metrics |
|---------|---------|
| User Growth | New users, churn rate, net growth |
| Revenue | MRR, ARR, growth rate |
| Engagement | DAU, MAU, retention |
| System Health | Uptime, latency, errors |
| Memory Quality | Accuracy, contradictions resolved |

**Export Options:**
- PDF presentation
- CSV data export
- Scheduled email reports

---

### User Analytics

In-depth analysis of user behavior, engagement patterns, and feature adoption. Include cohort analysis, retention metrics, and predictive models for churn prevention.

**Analytics Views:**
| View | Description |
|------|-------------|
| User Journey | Onboarding funnel visualization |
| Feature Heatmap | Usage by feature |
| Retention Curves | Cohort-based retention |
| Engagement Scores | User activity ranking |
| Churn Prediction | At-risk user identification |

---

### Team Performance

Comparative analysis of team efficiency metrics across support, operations, and affiliate channels. Include goal tracking, historical trends, and resource utilization insights.

---

## 9. Activity Log

Real-time feed of all administrative actions performed on the platform. Provides complete audit trail for security and compliance.

**Log Entry Format:**
```
Timestamp | User ID | Action | Details | IP Address
```

**Example Entries:**
| Timestamp | User | Action | Details |
|-----------|------|--------|---------|
| 2026-01-03 10:00:00 | admin_user | UPDATE_ROLE | Changed 'alice' to 'admin' |
| 2026-01-03 09:45:00 | admin_user | DELETE_USER | Removed 'spam_account' |
| 2026-01-03 09:30:00 | admin_user | TRIGGER_REFLECTION | Manual cycle initiated |
| 2026-01-03 09:15:00 | admin_user | EXTEND_TRIAL | Extended 'newuser' by 7 days |

**Filters:**
- Date range
- User ID
- Action type
- Severity level

---

## 10. Settings

Personal account settings, notification preferences, and interface customization options. Include security settings, two-factor authentication management, and session controls.

### Personal Account
- Profile information
- Password management
- Two-factor authentication
- Active sessions

### Notification Preferences
- Email notifications
- Push notifications
- Alert thresholds
- Digest frequency

### Interface Customization
- Theme (Dark/Light/System)
- Dashboard layout
- Default views
- Timezone

---

# Operations Interface

## 1. Dashboard (Home)

Operational overview showing user acquisition metrics, campaign performance, and affiliate activity. Include task prioritization widgets and goal tracking visualizations.

### Key Operational Metrics
- Daily Active Users (DAU)
- Weekly Active Users (WAU)
- User Acquisition by Channel
- Campaign Conversion Rates
- Affiliate Referral Volume

---

## 2. User Lifecycle

- **Onboarding Funnel**
- **Engagement Tracking**
- **Conversion Optimization**

---

### Onboarding Funnel

Visual representation of user journey from sign-up to activation with conversion rates at each step.

**Funnel Stages:**
| Stage | Target Rate |
|-------|-------------|
| Registration | 100% |
| Email Verification | > 85% |
| First Conversation | > 70% |
| Memory Created | > 55% |
| Return Visit | > 40% |
| Paid Conversion | > 15% |

---

### Engagement Tracking

Tools for monitoring user engagement levels and identifying at-risk accounts.

**Engagement Indicators:**
| Indicator | Weight |
|-----------|--------|
| Login Frequency | High |
| Session Duration | Medium |
| Features Used | High |
| Queries per Session | Medium |
| Memory Growth | High |

---

### Conversion Optimization

A/B testing tools and optimization strategies for improving trial-to-paid conversion rates.

---

## 3. Campaigns

- **Email Campaigns**
- **In-App Messaging**
- **Push Notifications**

---

### Email Campaigns

Management of automated and manual email campaigns for user engagement, re-activation, and promotions.

**Campaign Types:**
| Type | Trigger |
|------|---------|
| Welcome Series | New registration |
| Trial Expiring | 7/3/1 days remaining |
| Feature Announcement | New feature launch |
| Re-engagement | 7+ days inactive |
| Upgrade Promotion | High engagement detected |

---

### In-App Messaging

Configuration of contextual messages, tooltips, and promotional banners within the application.

---

### Push Notifications

Management of browser and mobile push notifications for time-sensitive communications.

---

## 4. Reports

- **Acquisition Analytics**
- **Conversion Reports**
- **Retention Analysis**

---

# Support Interface

## 1. Dashboard (Home)

Support-focused overview showing ticket queue, user assistance requests, and key support metrics.

### Support Metrics
- Open Tickets
- Average Response Time
- Resolution Rate
- Customer Satisfaction Score

---

## 2. User Lookup

- **Account Search**
- **User Actions**

---

### Account Search

Quick access to user account information for troubleshooting.

**Search By:**
- Username
- Email
- User ID

**Information Displayed:**
- Subscription status
- Recent activity
- Memory statistics
- Error logs

---

### User Actions

Available support actions for user accounts.

**Quick Actions:**
| Action | Description |
|--------|-------------|
| Reset Password | Send password reset link |
| Extend Trial | Add trial days |
| Apply Credit | Promotional credit |
| Clear Cache | Reset user cache |
| Export Data | User data download |
| Escalate | Forward to admin |

---

## 3. Ticket Management

- **Active Tickets**
- **Ticket Actions**

---

### Active Tickets

Queue of open support requests with priority sorting.

**Ticket Fields:**
| Field | Description |
|-------|-------------|
| Ticket ID | Unique identifier |
| User | Requester details |
| Category | Issue classification |
| Priority | Low / Medium / High / Urgent |
| Assigned | Agent handling |
| SLA Status | Time remaining |
| Created | Submission time |

---

### Ticket Actions

- Respond to user
- Change priority
- Reassign agent
- Escalate to admin
- Mark resolved
- Add internal note

---

## 4. Quick Actions

Library of one-click support actions for common requests such as password resets, trial extensions, and verification resends. Include usage tracking and customization options for individual support agents.

---

## 5. Knowledge Base

- **Common Solutions**
- **Template Library**
- **Feature Guides**

---

### Common Solutions

Repository of verified solutions to frequent issues, organized by category and searchable by keywords. Include usage statistics, effectiveness ratings, and continuous improvement workflows.

---

### Template Library

Collection of response templates for various support scenarios, categorized by issue type and tone. Include personalization variables, usage tracking, and effectiveness metrics.

---

### Feature Guides

Detailed documentation of platform features with troubleshooting tips, limitation notes, and use case examples. Include visual aids, step-by-step instructions, and frequently asked questions.

---

## 6. Reports

- **My Performance**
- **Team Metrics**
- **User Satisfaction**

---

### My Performance

Personal productivity and quality metrics with historical trends, goal tracking, and peer benchmarking. Include areas for improvement, recognition of achievements, and skill development recommendations.

---

### Team Metrics

Collaborative performance dashboard showing team-wide efficiency, quality, and workload distribution. Include shift coverage analysis, peak time identification, and resource allocation insights.

---

### User Satisfaction

Analysis of customer feedback, satisfaction scores, and sentiment trends across different issue types and resolution approaches. Include verbatim feedback and improvement recommendations.

---

## 7. Settings

Personal preferences for the support interface, notification settings, and workflow customization. Include availability status management, automated response settings, and interface personalization.

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
| `GET` | `/api/graph` | Get graph visualization |

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
- API rate limiting per tier

## Audit Trail
- All admin actions logged
- Immutable activity history
- Compliance-ready exports
- Retention: 2 years

---

*Document Version: 2.0 (UI-Aligned)*  
*Last Updated: January 3, 2026*  
*Platform: Reflective Memory Kernel*
