# Railway Deployment Guide

## Quick Deploy (Recommended)

Railway is a PaaS that can deploy directly from GitHub.

### Step 1: Prepare Your Repository

1. Push your code to GitHub (already done!)
2. Make sure `.env` and any secret files are in `.gitignore` ✅

### Step 2: Deploy on Railway

1. Go to [railway.app](https://railway.app)
2. Click **"Deploy from GitHub repo"**
3. Select your repository: `AkashKesav/Whitepaper`
4. Railway will detect it as a Docker project

### Step 3: Configure Services

Railway will create separate services for each container. Configure them:

---

## Service 1: DGraph Alpha
- **Root Directory**: `/`
- **Dockerfile**: `Dockerfile.monolith` (will use docker-compose)
- **Environment Variables**:
  ```
  DGRAPH_ADDRESS=localhost:9080
  ```

---

## Service 2: Monolith (Main App)
- **Root Directory**: `/`
- **Build Command**: `docker build -f Dockerfile.monolith -t $RAILWAY_VOLUME_DIRECTORY .`
- **Start Command**: `/monolith`

### Environment Variables (set in Railway dashboard):
```
DGRAPH_ADDRESS=<dgraph-service-url>:9080
NATS_URL=<nats-service-url>:4222
REDIS_ADDRESS=<redis-service-url>:6379
AI_SERVICES_URL=http://localhost:8000
QDRANT_URL=http://localhost:6333
JWT_SECRET=<generate-one>
PORT=9090
ALLOWED_ORIGINS=https://your-app.railway.app
```

---

## Service 3: AI Services
- **Root Directory**: `ai/`
- **Dockerfile**: `Dockerfile`

### Environment Variables:
```
NVIDIA_API_KEY=
OPENAI_API_KEY=
ANTHROPIC_API_KEY=
```

---

## Step 4: Generate JWT Secret

In Railway dashboard, for Monolith service:
1. Go to Variables tab
2. Add `JWT_SECRET` with a strong value
3. Generate using: `openssl rand -base64 32`

---

## Step 5: Deploy

Click **Deploy** on each service. Railway will:
- Build the Docker images
- Start the containers
- Provide public URLs

---

## Step 6: Access Your App

Your app will be available at:
```
https://your-app-name.railway.app
```

---

## Notes for Railway

1. **Persistent Storage**: Railway provides ephemeral storage. For production data persistence, consider Railway Volumes or external databases.

2. **DGraph on Railway**: DGraph needs persistent storage. You may want to use:
   - Railway's PostgreSQL instead (simpler)
   - Or external DGraph cloud service

3. **AI Services**: Users will add their own API keys via Settings UI, so you don't need to set default keys.

---

## Alternative: Use Railway Templates

For a simpler setup, you can create individual services:

1. **Monolith Service** (main app)
2. **Redis Service** (from Railway template)
3. **PostgreSQL Service** (from Railway template, instead of DGraph)

Then update environment variables to point to Railway's managed services.
