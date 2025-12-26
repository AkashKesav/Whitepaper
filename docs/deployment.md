<<<<<<< HEAD
# Deployment Guide

Guide for deploying the Reflective Memory Kernel in various environments.

## Quick Start

### Docker Compose (Development)

```bash
cd c:\Users\Akash Kesav\Documents\Whitepaper

# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

### Access Points

| Service | URL | Description |
|---------|-----|-------------|
| Chat UI | http://localhost:3000 | User interface |
| Memory Kernel | http://localhost:9000 | Kernel API |
| AI Services | http://localhost:8000 | AI endpoints |
| DGraph UI | http://localhost:8080 | Graph explorer |
| NATS Monitor | http://localhost:8222 | Stream monitor |

---

## Local Development

### Prerequisites

- Go 1.22+
- Python 3.11+
- Docker (for infrastructure)

### Step 1: Start Infrastructure

```bash
docker-compose up -d dgraph-zero dgraph-alpha nats redis ollama
```

### Step 2: Start AI Services

```bash
cd ai
pip install -r requirements.txt
python main.py
```

### Step 3: Start Memory Kernel

```bash
go run ./cmd/kernel
```

### Step 4: Start Front-End Agent

```bash
go run ./cmd/agent
```

---

## Docker Production Build

### Build Images

```bash
# Build Memory Kernel
docker build -t rmk-kernel:latest -f Dockerfile.kernel .

# Build Front-End Agent
docker build -t rmk-agent:latest -f Dockerfile.agent .

# Build AI Services
docker build -t rmk-ai:latest ./ai
```

### Run Production Stack

```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

---

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (1.28+)
- kubectl configured
- Helm 3.x

### Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rmk
```

### DGraph Deployment

```yaml
# dgraph.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dgraph-zero
  namespace: rmk
spec:
  serviceName: dgraph-zero
  replicas: 1
  selector:
    matchLabels:
      app: dgraph-zero
  template:
    spec:
      containers:
      - name: zero
        image: dgraph/dgraph:v24.0.0
        args:
          - dgraph
          - zero
          - --my=dgraph-zero-0.dgraph-zero:5080
        ports:
          - containerPort: 5080
          - containerPort: 6080
        volumeMounts:
          - name: data
            mountPath: /dgraph
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### Memory Kernel Deployment

```yaml
# kernel.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memory-kernel
  namespace: rmk
spec:
  replicas: 1  # Single instance for now
  selector:
    matchLabels:
      app: memory-kernel
  template:
    spec:
      containers:
      - name: kernel
        image: rmk-kernel:latest
        env:
          - name: DGRAPH_URL
            value: "dgraph-alpha:9080"
          - name: NATS_URL
            value: "nats://nats:4222"
          - name: REDIS_URL
            value: "redis:6379"
          - name: AI_SERVICES_URL
            value: "http://ai-services:8000"
        ports:
          - containerPort: 9000
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: memory-kernel
  namespace: rmk
spec:
  selector:
    app: memory-kernel
  ports:
    - port: 9000
      targetPort: 9000
```

### Front-End Agent Deployment

```yaml
# agent.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend-agent
  namespace: rmk
spec:
  replicas: 2  # Can scale horizontally
  selector:
    matchLabels:
      app: frontend-agent
  template:
    spec:
      containers:
      - name: agent
        image: rmk-agent:latest
        env:
          - name: MEMORY_KERNEL_URL
            value: "http://memory-kernel:9000"
          - name: NATS_URL
            value: "nats://nats:4222"
          - name: AI_SERVICES_URL
            value: "http://ai-services:8000"
        ports:
          - containerPort: 3000
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "300m"
---
apiVersion: v1
kind: Service
metadata:
  name: frontend-agent
  namespace: rmk
spec:
  type: LoadBalancer
  selector:
    app: frontend-agent
  ports:
    - port: 80
      targetPort: 3000
```

### Secrets

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: llm-secrets
  namespace: rmk
type: Opaque
stringData:
  openai-api-key: "sk-..."
  anthropic-api-key: "sk-..."
```

### Apply All

```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f dgraph.yaml
kubectl apply -f nats.yaml
kubectl apply -f redis.yaml
kubectl apply -f ai-services.yaml
kubectl apply -f kernel.yaml
kubectl apply -f agent.yaml
```

---

## Scaling Considerations

### Component Scaling

| Component | Scaling | Notes |
|-----------|---------|-------|
| Front-End Agent | Horizontal | Stateless, scales well |
| Memory Kernel | Single | Stateful reflection, single instance |
| AI Services | Horizontal | Stateless, scales with load |
| DGraph | Vertical first | Add replicas for HA |
| NATS | Cluster | 3-node cluster for HA |
| Redis | Cluster | Redis Cluster for HA |

### Memory Kernel Scaling (Future)

For true horizontal scaling of the Memory Kernel:

1. **Shard by User**: Each kernel handles a subset of users
2. **Message Routing**: NATS routes by user_id
3. **Leader Election**: For curation decisions

---

## Monitoring

### Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9000
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 9000
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Metrics (Future)

Prometheus metrics endpoints:
- `/metrics` on each service
- DGraph built-in metrics
- NATS built-in metrics

### Logging

All services log to stdout in JSON format:

```json
{"level":"info","ts":1699999999.999,"msg":"Chat response generated","latency":"234ms"}
```

---

## Backup & Recovery

### DGraph Backup

```bash
# Export data
curl -X POST localhost:8080/admin -d '{"query":"mutation { export(input: {}) { response { message } } }"}'

# Import data
dgraph bulk -f export.rdf.gz -s schema.dgraph
```

### Redis Backup

```bash
# Trigger RDB snapshot
redis-cli BGSAVE

# Copy RDB file
cp /data/dump.rdb /backup/
```

---

## Security

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: rmk-network-policy
  namespace: rmk
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: rmk
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              name: rmk
```

### TLS

For production, enable TLS on:
- Front-End Agent (via Ingress)
- Inter-service communication (mTLS)
- DGraph connections
=======
# Deployment

## Quick Start with Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Services

| Service | Port | Image |
|---------|------|-------|
| memory-kernel | 9000 | Local build |
| ai-services | 8000 | Local build |
| dgraph-alpha | 8080, 9080 | dgraph/dgraph:latest |
| dgraph-zero | 5080 | dgraph/dgraph:latest |
| nats | 4222, 8222 | nats:latest |
| redis | 6379 | redis:alpine |
| postgres | 5432 | postgres:15 |

## Docker Compose Configuration

```yaml
version: "3.8"

services:
  # DGraph Zero - Cluster Management
  dgraph-zero:
    image: dgraph/dgraph:latest
    command: dgraph zero --my=dgraph-zero:5080
    ports:
      - "5080:5080"
    volumes:
      - dgraph_zero:/dgraph

  # DGraph Alpha - Graph Database
  dgraph-alpha:
    image: dgraph/dgraph:latest
    command: dgraph alpha --my=dgraph-alpha:7080 --zero=dgraph-zero:5080
    ports:
      - "8080:8080"
      - "9080:9080"
    volumes:
      - dgraph_alpha:/dgraph
    depends_on:
      - dgraph-zero

  # NATS - Message Queue
  nats:
    image: nats:latest
    command: ["-js"]
    ports:
      - "4222:4222"
      - "8222:8222"

  # Redis - Cache
  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  # PostgreSQL - User Auth
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: rmk
      POSTGRES_PASSWORD: rmk_password
      POSTGRES_DB: rmk
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  # AI Services
  ai-services:
    build:
      context: ./ai
    ports:
      - "8000:8000"
    environment:
      - NVIDIA_API_KEY=${NVIDIA_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  # Memory Kernel (Unified)
  memory-kernel:
    build:
      context: .
      dockerfile: Dockerfile.unified
    ports:
      - "3000:3000"
      - "9000:9000"
    environment:
      - DGRAPH_URL=dgraph-alpha:9080
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis:6379
      - AI_SERVICES_URL=http://ai-services:8000
      - POSTGRES_URL=postgres://rmk:rmk_password@postgres:5432/rmk
    depends_on:
      - dgraph-alpha
      - nats
      - redis
      - postgres
      - ai-services

volumes:
  dgraph_zero:
  dgraph_alpha:
  redis_data:
  postgres_data:
```

## Environment Variables

Create `.env` file:

```env
# LLM Providers
NVIDIA_API_KEY=nvapi-...
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-...

# Infrastructure (defaults for Docker Compose)
DGRAPH_URL=dgraph-alpha:9080
NATS_URL=nats://nats:4222
REDIS_URL=redis:6379
AI_SERVICES_URL=http://ai-services:8000

# PostgreSQL
POSTGRES_URL=postgres://rmk:rmk_password@postgres:5432/rmk
```

## Building Images

### Memory Kernel (Go)
```dockerfile
# Dockerfile.unified
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o unified_system ./cmd/unified

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/unified_system .
COPY static/ static/
EXPOSE 3000 9000
CMD ["./unified_system"]
```

### AI Services (Python)
```dockerfile
# ai/Dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "main.py"]
```

## Health Checks

```bash
# Check all services
curl http://localhost:3000/api/health  # Agent
curl http://localhost:9000/api/health  # Kernel
curl http://localhost:8000/health      # AI Services

# DGraph health
curl http://localhost:8080/health

# NATS monitoring
curl http://localhost:8222/varz
```

## Scaling

### Horizontal Scaling
- AI Services: Stateless, scale horizontally
- Memory Kernel: Requires NATS for coordination
- DGraph: Use DGraph cluster mode

### Resource Requirements

| Service | CPU | Memory | Storage |
|---------|-----|--------|---------|
| memory-kernel | 0.5 | 512MB | - |
| ai-services | 1.0 | 1GB | - |
| dgraph-alpha | 1.0 | 2GB | 10GB |
| dgraph-zero | 0.25 | 256MB | 1GB |
| nats | 0.25 | 128MB | 1GB |
| redis | 0.25 | 256MB | 1GB |
| postgres | 0.5 | 512MB | 5GB |

## Production Considerations

### Security
- Enable TLS for all services
- Use secrets management for API keys
- Configure firewall rules
- Enable DGraph ACLs

### Monitoring
- Export metrics to Prometheus
- Configure alerting for service health
- Log aggregation with ELK or similar

### Backup
- Regular DGraph backups
- PostgreSQL daily backups
- Redis RDB snapshots

### High Availability
- DGraph cluster with 3+ Alphas
- NATS cluster mode
- Redis Sentinel or Cluster
- PostgreSQL replication
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
