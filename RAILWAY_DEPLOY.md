# Railway Deployment Guide

## Quick Start

Each service has its own `railway.toml` configuration. When adding a service from this repo, **set the Root Directory** to the service folder.

## Services Configuration

| Service      | Root Directory     | Dockerfile               | Public? |
| ------------ | ------------------ | ------------------------ | ------- |
| **backend**  | `/backend`         | `../Dockerfile.backend`  | ❌ No   |
| **frontend** | `/frontend`        | `../Dockerfile.frontend` | ✅ Yes  |
| **dgraph**   | `/services/dgraph` | `Dockerfile`             | ❌ No   |
| **nats**     | `/services/nats`   | `Dockerfile`             | ❌ No   |
| **qdrant**   | `/services/qdrant` | `Dockerfile`             | ❌ No   |

## Deployment Steps

### 1. Create Project

- Railway Dashboard → **New Project** → **Empty Project**

### 2. Add Redis Plugin

- Click **+ New** → **Database** → **Redis**

### 3. Add Each Service

For each service (dgraph, nats, qdrant, backend, frontend):

1. Click **+ New** → **GitHub Repo** → Select `AkashKesav/Whitepaper`
2. Set **Root Directory** to the service folder (e.g., `/services/dgraph`)
3. Railway will auto-detect `railway.toml` in that folder
4. Click **Deploy**

### 4. Configure Backend Variables

In the **backend** service, go to Variables and add:

```
DGRAPH_ADDRESS=dgraph.railway.internal:9080
NATS_URL=nats://nats.railway.internal:4222
QDRANT_URL=http://qdrant.railway.internal:6333
JWT_SECRET=your-32-char-secret-minimum-length
```

Click **Add Reference** → Select **Redis** → Choose `REDIS_URL`

### 5. Generate Frontend Domain

Only for **frontend** service:

- Settings → Networking → **Generate Domain**
- This is your public URL!

## Recommended Deploy Order

1. Redis (plugin)
2. dgraph
3. nats
4. qdrant
5. backend
6. frontend

## Internal Networking

Services communicate via Railway's private network:

- `<service-name>.railway.internal:<port>`
- Only frontend should have a public domain
