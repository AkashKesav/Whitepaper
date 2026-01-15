# Railway Deployment Guide

This application can be deployed to Railway as a multi-service project.

## Architecture on Railway

```
┌─────────────────────────────────────────────────────────┐
│                     Railway Project                      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │  Monolith   │  │ AI Services │  │  Redis (Plugin) │ │
│  │  (Go+React) │  │  (Python)   │  │                 │ │
│  │  Port 8080  │  │  Port 8000  │  │                 │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│  ┌──────┴────────────────┴───────────────────┴────────┐ │
│  │              Internal Railway Network              │ │
│  └────────────────────────────────────────────────────┘ │
│                                                         │
│  External Services (Managed):                           │
│  • DGraph Cloud (dgraph.io) or Upstash                  │
│  • Qdrant Cloud (qdrant.io)                             │
│  • Ollama on RunPod/Modal (optional)                    │
└─────────────────────────────────────────────────────────┘
```

## Quick Deploy

### Option 1: One-Click Deploy (Single Service - Recommended)

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template)

### Option 2: Manual Setup

1. **Create Railway Project**

   ```bash
   npm install -g @railway/cli
   railway login
   railway init
   ```

2. **Add Redis Plugin**

   - In Railway dashboard → Add → Database → Redis
   - Copy `REDIS_URL` to your service

3. **Deploy Monolith**

   ```bash
   railway up
   ```

4. **Set Environment Variables** (in Railway dashboard):

   ```
   # Required
   JWT_SECRET=your-secure-secret-at-least-32-characters

   # Redis (from Railway Redis plugin)
   REDIS_ADDRESS=${REDIS_URL}

   # For DGraph (use DGraph Cloud or self-host)
   DGRAPH_ADDRESS=your-dgraph-cloud-endpoint:9080

   # For AI Services (if deploying separately)
   AI_SERVICES_URL=https://your-ai-service.railway.app

   # For Qdrant (use Qdrant Cloud)
   QDRANT_URL=https://your-cluster.qdrant.io:6333

   # For LLM (API keys)
   OPENAI_API_KEY=sk-...
   ANTHROPIC_API_KEY=sk-...
   ```

## Service Configurations

### Monolith (Main Service)

- **Root Directory:** `/`
- **Dockerfile:** `Dockerfile.monolith`
- **Port:** `8080` (auto-detected)
- **Health Check:** `/health`

### AI Services (Optional - separate service)

- **Root Directory:** `/ai`
- **Dockerfile:** `ai/Dockerfile`
- **Port:** `8000`

## External Managed Services

### DGraph Cloud

1. Sign up at https://cloud.dgraph.io
2. Create a backend
3. Copy the gRPC endpoint to `DGRAPH_ADDRESS`

### Qdrant Cloud

1. Sign up at https://qdrant.io
2. Create a cluster
3. Copy the endpoint to `QDRANT_URL`

### Alternative: Self-host on Railway

You can also deploy DGraph and Qdrant as Docker services on Railway, but managed services are recommended for production.

## Environment Variables Reference

| Variable            | Required | Description                           |
| ------------------- | -------- | ------------------------------------- |
| `PORT`              | Auto     | Port to listen on (Railway sets this) |
| `JWT_SECRET`        | Yes      | Secret for JWT tokens (min 32 chars)  |
| `REDIS_ADDRESS`     | Yes      | Redis connection string               |
| `DGRAPH_ADDRESS`    | Yes      | DGraph gRPC endpoint                  |
| `QDRANT_URL`        | No       | Qdrant HTTP endpoint                  |
| `AI_SERVICES_URL`   | No       | AI services endpoint                  |
| `OLLAMA_URL`        | No       | Ollama endpoint for embeddings        |
| `OPENAI_API_KEY`    | No       | OpenAI API key                        |
| `ANTHROPIC_API_KEY` | No       | Anthropic API key                     |
| `ALLOWED_ORIGINS`   | No       | CORS origins (comma-separated)        |

## Scaling

Railway supports horizontal scaling. The monolith is stateless and can be scaled horizontally. Just increase the replica count in Railway dashboard.

## Monitoring

- Railway provides built-in logs and metrics
- Health endpoint: `/health`
- Debug static files: `/debug-static`

## Troubleshooting

### Frontend not loading

- Check if static files are built: `/debug-static`
- Ensure `STATIC_DIR` is not overridden

### API errors

- Check logs in Railway dashboard
- Verify environment variables are set
- Ensure DGraph and Redis are accessible

### Build failures

- Check Node.js dependencies: `npm ci` may need `--legacy-peer-deps`
- Go mod download may timeout - retries usually work
