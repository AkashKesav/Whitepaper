# Reflective Memory Kernel - Platform Panels Guide

> User interface documentation for trial users, paid users, admins, and support teams.

---

## Overview

The platform consists of multiple panels designed for different user roles and tiers. This document outlines the access levels, features, and capabilities for each panel type.

---

## 1. User Tiers & Access Matrix

### Tier Definitions

| Tier | Description | Access Level |
|------|-------------|--------------|
| **Trial User** | Free tier with limited features | Basic panels only |
| **Paid User** | Pro/Team subscription | Full user panels + collaboration |
| **Admin** | Platform administrators | All panels + system controls |
| **Super Admin** | Internal platform operators | Full access + support tools |

### Feature Access Matrix

| Feature | Trial | Paid (Pro) | Paid (Team) | Admin |
|---------|:-----:|:----------:|:-----------:|:-----:|
| Chat Interface | ✅ | ✅ | ✅ | ✅ |
| Graph Visualization | ✅ (limited) | ✅ | ✅ | ✅ |
| Document Ingestion | 3/month | Unlimited | Unlimited | ✅ |
| Memory Storage | 500 nodes | 10K nodes | 50K nodes | ✅ |
| Groups/Workspaces | ❌ | 1 personal | 10 shared | ✅ |
| API Keys | ❌ | 1 key | 5 keys | ✅ |
| Admin Panel | ❌ | ❌ | ❌ | ✅ |
| System Controls | ❌ | ❌ | ❌ | ✅ |

---

## 2. Trial User Panel

### 2.1 Dashboard (Limited)

**Location**: `/dashboard`

**Features Available**:
- Graph View (up to 500 nodes)
- Basic search functionality
- View-only insights panel
- System status indicator

**Restrictions**:
- No Kernel Operations (spreading activation, community detection)
- No temporal decay controls
- Limited to 30-day data retention

### 2.2 Chat Interface

**Location**: `/chat`

**Features Available**:
- Conversational AI interface
- Basic memory recall
- Single namespace (personal only)

**Restrictions**:
- No group/workspace context switching
- Rate limited to 50 queries/day
- No document context

### 2.3 Upgrade Prompts

Trial users see upgrade prompts when:
- Attempting to access locked features
- Reaching storage limits
- Trying to create groups
- Using API integration features

---

## 3. Paid User Panel (Pro Tier)

### 3.1 Full Dashboard

**Location**: `/dashboard`

**All Features**:
- Complete Graph View with all nodes
- Kernel Operations panel:
  - Spreading Activation
  - Community Detection (Leiden algorithm)
  - Temporal Decay controls
- Real-time console output
- Recent Insights display
- Top Entities ranking
- Ingestion status monitoring

### 3.2 Chat Interface (Full)

**Location**: `/chat`

**Features**:
- Unlimited queries
- Full memory context
- Conversation history
- Namespace switching (personal + group context)

### 3.3 Document Ingestion

**Location**: `/ingestion`

**Features**:
- File upload (PDF, TXT, DOCX, JSON)
- Batch processing
- Entity extraction preview
- Progress tracking
- Processing status

### 3.4 Settings Panel

**Location**: `/settings`

**Sections**:

| Section | Description |
|---------|-------------|
| **Profile** | Username, email, avatar management |
| **Notifications** | Email, push, insight alerts, updates toggles |
| **API Keys** | Generate/manage production API keys |
| **Appearance** | Theme selection (dark/light/system) |
| **Privacy & Security** | 2FA, password change |
| **Data Management** | Export data, delete account |

---

## 4. Paid User Panel (Team Tier)

### 4.1 All Pro Features Plus:

### 4.2 Groups/Workspaces

**Location**: `/groups`

**Features**:
- Create and manage workspaces
- Invite team members by username
- Share links with usage limits
- Role management (owner/admin/member)
- Shared memory space across team

**Collaboration Capabilities**:
- Workspace invitation system
- Pending invitation management
- Member list with roles
- Shareable join links

### 4.3 Advanced Settings

Additional settings for Team tier:
- Workspace management
- Member permissions
- Billing management
- Usage analytics

---

## 5. Admin Panel

**Location**: `/admin`

**Access**: Admin role users only (route protected)

### 5.1 Users Tab

**Endpoint**: `GET /api/admin/users`

**Capabilities**:
- List all registered users
- View user roles (admin/user)
- Promote/demote user roles
- Delete user accounts

**UI Elements**:
- User list with avatar icons
- Role badges (admin/user)
- Action buttons (Promote/Demote, Delete)
- Loading states

### 5.2 System Tab

**Endpoints**:
- `GET /api/admin/system/stats`
- `POST /api/admin/system/reflection`

**Capabilities**:
- View system statistics:
  - Total users count
  - Admin count
  - Kernel stats
  - Cache stats
  - Uptime
- Trigger manual reflection cycles

**UI Elements**:
- Statistics cards (Total Users, Admins)
- Timestamp display
- Reflection trigger button
- System health indicators

### 5.3 Groups Tab

**Endpoints**:
- `GET /api/admin/groups`
- `DELETE /api/admin/groups/{id}`

**Capabilities**:
- View all system groups
- Delete groups (coming soon in UI)

**Current Status**: Placeholder - redirects to regular groups interface

### 5.4 Activity Tab

**Endpoint**: `GET /api/admin/activity`

**Capabilities**:
- View admin activity log
- Track administrative actions
- Audit trail with timestamps

**Log Entry Format**:
```json
{
  "timestamp": "2024-01-03T10:00:00Z",
  "user_id": "admin_username",
  "action": "UPDATE_ROLE",
  "details": "Changed user 'alice' role to 'admin'"
}
```

---

## 6. Support Panel (Proposed)

### 6.1 Customer Dashboard

**Proposed Location**: `/support` or `/admin/support`

**Features to Implement**:
- Customer account lookup
- Subscription status view
- Usage metrics per user
- Support ticket integration
- Account impersonation (read-only)

### 6.2 Diagnostics Tools

**Proposed Capabilities**:
- Query user memory health
- View Pre-Cortex cache stats per user
- Check ingestion pipeline status
- Review error logs
- Trigger data exports on behalf of user

### 6.3 Billing Support

**Proposed Capabilities**:
- View subscription history
- Process refunds
- Apply promotional credits
- Upgrade/downgrade accounts
- View payment failures

---

## 7. API Endpoints Reference

### Admin Endpoints

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

### User Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/chat` | Send chat message |
| `GET` | `/api/conversations` | List conversations |
| `POST` | `/api/upload` | Upload document |
| `GET` | `/api/groups` | List user's groups |
| `POST` | `/api/groups` | Create group |
| `GET` | `/api/stats` | User memory stats |

---

## 8. Implementation Roadmap

### Completed ✅
- [x] Admin Panel (Users, System, Activity tabs)
- [x] User Dashboard with Kernel Operations
- [x] Settings with 6 sections
- [x] Groups/Workspaces basic UI
- [x] Role-based route protection

### In Progress 🔄
- [ ] Groups Admin management in Admin Panel
- [ ] Usage analytics dashboard
- [ ] Billing integration

### Planned 📋
- [ ] Support Panel for customer service
- [ ] Trial-to-Paid upgrade flow
- [ ] Usage limits enforcement
- [ ] Billing management UI
- [ ] API key management expansion
- [ ] Data export automation

---

## 9. Security Considerations

### Authentication
- JWT-based authentication
- Token stored in AuthContext
- Automatic redirect for unauthenticated users

### Authorization
- Admin routes protected via `AdminProtectedRoute` component
- Role checked via `isAdmin` from AuthContext
- Backend validates JWT + role on every admin endpoint

### Audit Trail
- All admin actions logged to Redis
- Activity log accessible in Admin Panel
- Timestamps and user IDs recorded

---

## Appendix: File References

| Panel | Frontend File | Backend Handler |
|-------|---------------|-----------------|
| Admin | `frontend/src/pages/Admin.tsx` | `internal/agent/admin_handlers.go` |
| Dashboard | `frontend/src/pages/Dashboard.tsx` | `internal/agent/server.go` |
| Settings | `frontend/src/pages/Settings.tsx` | `internal/agent/server.go` |
| Groups | `frontend/src/pages/Groups.tsx` | `internal/agent/server.go`, `internal/kernel/kernel.go` |
| Chat | `frontend/src/pages/Chat.tsx` | `internal/agent/server.go` |
| Auth | `frontend/src/pages/Auth.tsx` | `internal/agent/server.go` |
