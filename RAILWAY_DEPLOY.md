# Railway Deployment Guide

## All Dockerfiles in Root

| Service      | Dockerfile            | Public?    |
| ------------ | --------------------- | ---------- |
| **backend**  | `Dockerfile.backend`  | ❌ Private |
| **frontend** | `Dockerfile.frontend` | ✅ Public  |
| **dgraph**   | `Dockerfile.dgraph`   | ❌ Private |
| **nats**     | `Dockerfile.nats`     | ❌ Private |
| **qdrant**   | `Dockerfile.qdrant`   | ❌ Private |

---

## Quick Deploy Steps

### 1. Create Project

Railway → **New Project** → **Empty Project**

### 2. Add Redis

**+ New** → **Database** → **Redis**

### 3. Add Each Service

For each service (dgraph, nats, qdrant, backend, frontend):

1. Click **+ New** → **GitHub Repo** → Select `AkashKesav/Whitepaper`
2. Go to **Settings** → **Build**
3. Set **Dockerfile Path** to the correct file (e.g., `Dockerfile.dgraph`)
4. Click **Deploy**

### 4. Set Backend Variables

In **backend** service → **Variables**:

```
DGRAPH_ADDRESS=dgraph.railway.internal:9080
NATS_URL=nats://nats.railway.internal:4222
QDRANT_URL=http://qdrant.railway.internal:6333
JWT_SECRET=yourSecretKeyAtLeast32Characters
```

Add Reference → Redis → `REDIS_URL`

### 5. Generate Frontend Domain

**frontend** service → **Settings** → **Networking** → **Generate Domain**

---

## Deploy Order

1. Redis (plugin)
2. dgraph
3. nats
4. qdrant
5. backend (set variables)
6. frontend (generate domain)
