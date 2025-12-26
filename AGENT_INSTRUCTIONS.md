# Agent Continuation Instructions

This document provides context and instructions for continuing development on this project.

## Project Overview

**Reflective Memory Kernel (RMK)** - An AI memory system with persistent context and adaptive knowledge graphs.

### Architecture
- **Go Backend** (`internal/`) - Memory Kernel, Agent, Graph operations
- **Python AI Services** (`ai/`) - Document ingestion, LLM routing, Vector indexing
- **React Frontend** (`frontend/`) - Dashboard, Chat, Groups, Ingestion pages
- **DGraph** - Graph database for knowledge storage
- **Redis** - Hot cache for fast retrieval
- **NATS** - Message broker for event-driven ingestion

## Current State

### Working Features ✅
- User Registration/Login with JWT authentication
- Chat with AI (uses NVIDIA DeepSeek API)
- Document ingestion with Vector-Native hierarchical indexing
- Groups management
- Knowledge Graph storage in DGraph
- Reflection engine for memory consolidation

### Known Issues ⚠️

#### API Timeout Issues
Some protected endpoints (`/api/dashboard/stats`, `/api/list-groups`) have slow response times or timeout issues related to DGraph queries. Fixes have been implemented but need verification:

1. **Handler-level timeouts** (`dashboard.go`, `server.go`)
   - 5-second fallback using goroutine+channel pattern
   - Returns fallback data instead of blocking

2. **gRPC interceptor timeout** (`graph/client.go`)
   - 10-second per-call timeout at gRPC level
   - Added unary interceptor for all DGraph calls

3. **Files modified for timeout fixes:**
   - `internal/agent/dashboard.go` - GetDashboardStats, GetVisualGraph
   - `internal/agent/server.go` - handleCreateGroup, handleListGroups
   - `internal/graph/client.go` - gRPC unary interceptor

## Development Setup

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Node.js 18+
- Python 3.11+

### Running the Project

```powershell
# Windows (PowerShell)
cd Whitepaper
docker compose up -d

# Check service health
docker compose ps
curl http://localhost:3000/health
```

```bash
# Linux/macOS
cd Whitepaper
docker compose up -d
docker compose ps
curl http://localhost:3000/health
```

### Service Ports
- **3000** - Go Backend (unified-system)
- **5173** - React Frontend
- **8001** - Python AI Services
- **9080** - DGraph Alpha (internal)
- **8080** - DGraph HTTP (internal)

## Next Steps / TODO

### High Priority
1. **Verify API timeout fixes** - Test `/api/dashboard/stats` and `/api/list-groups` endpoints
2. **Test frontend integration** - Ensure all pages work correctly
3. **Fix any remaining DGraph query performance issues**

### Medium Priority
4. **Implement conversation history endpoint** - Currently missing `/api/conversations` for chat history
5. **Add vector tree persistence to DGraph** - Store hierarchical vector data
6. **Improve error handling** - Add better error messages and recovery

### Low Priority
7. **Add comprehensive tests** - Unit and integration tests
8. **Performance optimization** - Profile and optimize slow queries
9. **Documentation** - API documentation with examples

## Key Files to Review

### Backend (Go)
- `internal/agent/server.go` - HTTP handlers
- `internal/agent/dashboard.go` - Dashboard endpoints
- `internal/graph/client.go` - DGraph client
- `internal/kernel/kernel.go` - Memory Kernel core

### AI Services (Python)
- `ai/main.py` - FastAPI endpoints
- `ai/document_ingester.py` - Document processing
- `ai/vector_index/indexer.py` - Vector tree builder

### Frontend (React/TypeScript)
- `frontend/src/pages/` - Page components
- `frontend/src/lib/api.ts` - API client

## API Endpoints Reference

### Public
- `GET /health` - Health check
- `POST /api/register` - User registration
- `POST /api/login` - User login

### Protected (requires JWT)
- `POST /api/chat` - Send chat message
- `GET /api/dashboard/stats` - Dashboard statistics
- `GET /api/dashboard/graph` - Graph visualization data
- `GET /api/dashboard/ingestion` - Ingestion stats
- `POST /api/groups` - Create group
- `GET /api/list-groups` - List user groups
- `POST /api/upload` - Upload document

## Environment Variables

```env
DGRAPH_URL=dgraph-alpha:9080
NATS_URL=nats://nats:4222
REDIS_URL=localhost:6379
AI_SERVICES_URL=http://ai-services:8001
JWT_SECRET=dev-secret-change-in-production
```

## Testing Commands

```powershell
# Windows PowerShell - Test APIs
$token = (Invoke-RestMethod -Uri "http://localhost:3000/api/login" -Method POST -Body '{"username":"test","password":"test123"}' -ContentType "application/json").token
Invoke-RestMethod -Uri "http://localhost:3000/api/dashboard/stats" -Headers @{Authorization="Bearer $token"}
```

```bash
# Linux/macOS - Test APIs
TOKEN=$(curl -s -X POST http://localhost:3000/api/login -H "Content-Type: application/json" -d '{"username":"test","password":"test123"}' | jq -r '.token')
curl -s http://localhost:3000/api/dashboard/stats -H "Authorization: Bearer $TOKEN" | jq .
```

## Troubleshooting

### Containers not starting
```bash
docker compose down
docker compose up -d
docker compose logs unified-system
```

### DGraph connection issues
```bash
docker compose logs dgraph-alpha
# Check if DGraph is healthy before unified-system starts
```

### API timeouts
- Check container logs for timeout warnings
- Verify DGraph is responding: `curl http://localhost:8080/health`
- The fallback timeouts (5s) should return empty/fallback data

## Git Workflow

```bash
# Pull latest changes
git pull origin main

# Create feature branch
git checkout -b feature/my-feature

# Commit and push
git add .
git commit -m "Description of changes"
git push origin feature/my-feature
```
