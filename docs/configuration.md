# Configuration

Complete configuration reference for all Reflective Memory Kernel components.

## Environment Variables

### Front-End Agent

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | HTTP server port |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `MEMORY_KERNEL_URL` | `http://localhost:9000` | Memory Kernel API URL |
| `AI_SERVICES_URL` | `http://localhost:8000` | AI Services API URL |

### Memory Kernel

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9000` | HTTP server port |
| `DGRAPH_URL` | `localhost:9080` | DGraph Alpha gRPC address |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `REDIS_URL` | `localhost:6379` | Redis server address |
| `AI_SERVICES_URL` | `http://localhost:8000` | AI Services API URL |

### AI Services

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8000` | HTTP server port |
| `OPENAI_API_KEY` | - | OpenAI API key (optional) |
| `ANTHROPIC_API_KEY` | - | Anthropic API key (optional) |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |

---

## Configuration Files

### .env Example

```bash
# LLM Providers (at least one recommended)
OPENAI_API_KEY=sk-proj-your-key-here
ANTHROPIC_API_KEY=sk-ant-your-key-here

# Infrastructure (use defaults for Docker Compose)
DGRAPH_URL=dgraph-alpha:9080
NATS_URL=nats://nats:4222
REDIS_URL=redis:6379

# AI Services
OLLAMA_HOST=http://ollama:11434
```

---

## Tuning Parameters

### Reflection Engine

```go
type Config struct {
    // How often to run reflection cycles
    ReflectionInterval time.Duration  // Default: 5 * time.Minute
    
    // Activation decay rate per day (0.0 - 1.0)
    ActivationDecayRate float64       // Default: 0.05 (5%)
    
    // Minimum batch size before processing
    MinReflectionBatch int            // Default: 10
    
    // Maximum batch size per cycle
    MaxReflectionBatch int            // Default: 100
}
```

### Ingestion Pipeline

```go
type Config struct {
    // Batch size for entity processing
    IngestionBatchSize int            // Default: 50
    
    // How often to flush pending ingestions
    IngestionFlushInterval time.Duration  // Default: 10 * time.Second
}
```

### Activation

```go
type ActivationConfig struct {
    // Decay rate per day (0.0 - 1.0)
    DecayRate float64             // Default: 0.05
    
    // Boost per access (0.0 - 1.0)
    BoostPerAccess float64        // Default: 0.10
    
    // Minimum activation before pruning consideration
    MinActivation float64         // Default: 0.01
    
    // Maximum activation level
    MaxActivation float64         // Default: 1.0
    
    // Threshold for core identity promotion
    CoreIdentityThreshold float64 // Default: 0.8
}
```

---

## Performance Tuning

### Memory Kernel

| Parameter | Low Memory | Balanced | High Performance |
|-----------|------------|----------|------------------|
| `ReflectionInterval` | 15m | 5m | 1m |
| `MaxReflectionBatch` | 50 | 100 | 500 |
| `DecayRate` | 0.1 | 0.05 | 0.02 |

### Front-End Agent

| Parameter | Low Latency | Balanced | High Reliability |
|-----------|-------------|----------|------------------|
| MK Timeout | 1s | 2s | 5s |
| AI Timeout | 5s | 10s | 30s |

### DGraph

For production workloads, tune DGraph:

```yaml
dgraph alpha:
  --cache_mb=2048           # Memory cache in MB
  --posting_list_cache_mb=1024
  --badger.compression=zstd  # Enable compression
```

### Redis

```conf
maxmemory 256mb
maxmemory-policy allkeys-lru
```

### NATS

```conf
max_payload: 1048576  # 1MB max message size
max_pending: 67108864 # 64MB max pending
```

---

## Model Selection

### Recommended Models by Task

| Task | Speed Priority | Quality Priority | Cost Priority |
|------|----------------|------------------|---------------|
| Extraction | `gpt-3.5-turbo` | `gpt-4` | `llama3.2:3b` |
| Curation | `gpt-3.5-turbo` | `claude-3-haiku` | `llama3.2:3b` |
| Synthesis | `gpt-4o-mini` | `gpt-4` | `llama3.2:8b` |
| Generation | `gpt-4o-mini` | `gpt-4` | `llama3.2:8b` |

### Ollama Model Setup

```bash
# Pull recommended models
ollama pull llama3.2:3b   # Fast, lightweight
ollama pull llama3.2:8b   # Balanced
ollama pull mistral       # Good for coding
```

---

## Resource Requirements

### Minimum (Development)

| Component | CPU | Memory | Storage |
|-----------|-----|--------|---------|
| DGraph Zero | 0.5 | 256MB | 1GB |
| DGraph Alpha | 1 | 1GB | 5GB |
| NATS | 0.2 | 128MB | 1GB |
| Redis | 0.2 | 256MB | 100MB |
| Memory Kernel | 0.5 | 256MB | - |
| Front-End Agent | 0.2 | 128MB | - |
| AI Services | 0.5 | 512MB | - |
| Ollama | 2+ | 4GB+ | 10GB+ |

### Recommended (Production)

| Component | CPU | Memory | Storage |
|-----------|-----|--------|---------|
| DGraph Zero | 2 | 1GB | 10GB SSD |
| DGraph Alpha | 4 | 4GB | 50GB SSD |
| NATS | 1 | 512MB | 5GB SSD |
| Redis | 1 | 1GB | 1GB |
| Memory Kernel | 2 | 1GB | - |
| Front-End Agent | 1 | 512MB | - |
| AI Services | 2 | 2GB | - |
| Ollama | 4+ | 16GB+ | 50GB+ |

---

## Logging Configuration

### Go Services (zap)

```go
// Production logger
logger, _ := zap.NewProduction()

// Development logger (more verbose)
logger, _ := zap.NewDevelopment()
```

Log levels:
- `DEBUG` - Detailed tracing
- `INFO` - Normal operations
- `WARN` - Recoverable issues
- `ERROR` - Failures

### Python Services (uvicorn)

```bash
# Development
uvicorn main:app --log-level debug

# Production
uvicorn main:app --log-level warning
```

---

## Feature Flags (Future)

```yaml
features:
  synthesis:
    enabled: true
    interval: 5m
  anticipation:
    enabled: true
    min_frequency: 3
  curation:
    enabled: true
    auto_archive: true
  prioritization:
    enabled: true
    decay_enabled: true
```
