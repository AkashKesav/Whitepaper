# Oracle Compute Infrastructure Deployment Guide

This guide covers deploying the Reflective Memory Kernel (RMK) to Oracle Cloud Infrastructure (OCI).

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [OCI Instance Setup](#oci-instance-setup)
3. [Environment Configuration](#environment-configuration)
4. [Deployment](#deployment)
5. [NVIDIA NIM Integration](#nvidia-nim-integration)
6. [Security Considerations](#security-considerations)
7. [Monitoring and Maintenance](#monitoring-and-maintenance)

## Prerequisites

### Oracle Cloud Account

- Active Oracle Cloud Infrastructure account with appropriate permissions
- Access to create Compute instances, Block Volumes, and configure networking

### Required Tools

```bash
# OCI CLI (optional but recommended)
oci --version

# Docker and Docker Compose
docker --version
docker-compose --version

# Git
git --version
```

### NVIDIA NIM Account

- NVIDIA account with API key from [build.nvidia.com](https://build.nvidia.com)
- The NIM API provides cloud-based LLM inference (replacing local Ollama)

## OCI Instance Setup

### 1. Create Compute Instance

1. Navigate to **Compute > Instances** in OCI Console
2. Click **Create Instance**
3. Configure:
   - **Name**: `rmk-production`
   - **Shape**: `VM.Standard.E4.Flex` (or similar with at least 4 OCPUs, 16GB RAM)
   - **Operating System**: Oracle Linux or Ubuntu (22.04 LTS recommended)
   - **SSH Key**: Add your public SSH key
4. **Networking**:
   - VCN: Create or use existing
   - Subnet: Public subnet with Internet Gateway
   - Assign Public IP: Yes
5. **Boot Volume**: 50GB minimum

### 2. Configure Security Lists / Firewall

Add inbound rules to your subnet's Security List:

| Protocol | Source | Destination Port | Description |
|----------|--------|------------------|-------------|
| TCP      | 0.0.0.0/0 | 22 | SSH |
| TCP      | 0.0.0.0/0 | 80 | HTTP |
| TCP      | 0.0.0.0/0 | 443 | HTTPS |
| TCP      | 0.0.0.0/0 | 9090 | RMK API |

### 3. Attach Block Volumes (Optional but Recommended)

For production, attach encrypted block volumes for persistent data:

| Volume | Mount Point | Size |
|--------|-------------|------|
| dgraph-data | /data/dgraph | 100GB |
| redis-data | /data/redis | 20GB |
| qdrant-data | /data/qdrant | 100GB |

## Environment Configuration

### 1. SSH into the Instance

```bash
ssh opc@<your-public-ip>
# or for Ubuntu:
ssh ubuntu@<your-public-ip>
```

### 2. Install Docker

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

### 3. Clone Repository

```bash
cd /opt
sudo git clone <your-repo-url> rmk
cd rmk
```

### 4. Configure Environment

```bash
# Copy the Oracle environment template
cp .env.oracle .env

# Edit with your values
nano .env
```

**Required Changes in `.env`:**

```bash
# Update these values
NIM_API_KEY=your-actual-nim-api-key-here
JWT_SECRET=<generate-with-openssl-rand-base64-32>
FRONTEND_URL=https://your-domain-or-ip
API_BASE_URL=https://your-domain-or-ip/api

# Optional: Use OCI services
# REDIS_HOST=<oci-redis-endpoint>
# DGRAPH_HOST=<oci-dgraph-endpoint>
```

Generate a secure JWT secret:

```bash
openssl rand -base64 32
```

## Deployment

### 1. Build and Start Services

```bash
# Build images
docker-compose -f docker-compose.oracle.yml build

# Start services
docker-compose -f docker-compose.oracle.yml up -d
```

### 2. Verify Health

```bash
# Check all containers are running
docker-compose -f docker-compose.oracle.yml ps

# Check health endpoint
curl http://localhost:9090/health
```

Expected response:
```json
{"status":"healthy"}
```

### 3. Bootstrap Initial Admin User

```bash
curl -X POST http://localhost:9090/api/bootstrap \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "<secure-password>",
    "email": "admin@your-domain.com"
  }'
```

Save the returned token for future API calls.

## NVIDIA NIM Integration

The Oracle deployment uses NVIDIA NIM for LLM inference instead of Ollama:

### Key Differences from Development

| Development | Production (Oracle) |
|-------------|---------------------|
| Ollama (local) | NVIDIA NIM (cloud API) |
| No API key required | NIM API key required |
| Limited by local GPU | Scaled cloud resources |

### Configure NIM in Frontend

1. Navigate to **Settings > AI Providers**
2. Enter your NVIDIA NIM API key (from [build.nvidia.com](https://build.nvidia.com))
3. Click **Test Connection** to verify
4. Click **Save** to store the key

### Available Models

| Model | Type | Use Case |
|-------|------|----------|
| `meta/llama-3.1-405b-instruct` | Chat | General chat, reasoning |
| `meta/llama-3.1-70b-instruct` | Chat | Faster chat responses |
| `nvidia/nv-embedqa-mistral-7b` | Embeddings | Vector embeddings |

## Security Considerations

### 1. SSL/TLS Termination

For production, use a reverse proxy with SSL:

**Option A: Nginx**

```bash
sudo apt install nginx certbot python3-certbot-nginx -y

# Configure Nginx
sudo nano /etc/nginx/sites-available/rmk
```

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        return 301 https://$server_name$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    location /api/ {
        proxy_pass http://localhost:9090/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location / {
        proxy_pass http://localhost:3000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Option B: OCI Load Balancer**

1. Create an OCI Load Balancer
2. Add backend set for port 9090
3. Configure SSL certificate
4. Update security lists

### 2. Oracle Cloud Vault

For production, use OCI Vault for secrets:

```bash
# Install OCI CLI
oci secrets secret-bundle create ...
```

### 3. Firewall Rules

Restrict access to management ports:

```bash
# Only allow SSH from specific IPs
sudo iptables -A INPUT -p tcp --dport 22 -s <your-ip> -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 22 -j DROP
```

### 4. OCI WAF (Optional)

Enable OCI Web Application Firewall for:
- DDoS protection
- SQL injection prevention
- XSS protection

## Monitoring and Maintenance

### 1. Container Monitoring

```bash
# View logs
docker-compose -f docker-compose.oracle.yml logs -f

# Restart services
docker-compose -f docker-compose.oracle.yml restart monolith
```

### 2. Resource Monitoring

Use OCI Monitoring service:
- Create metrics for CPU, memory, disk
- Set up alarms for high usage
- Configure notifications

### 3. Backup Strategy

**DGraph Backups:**

```bash
# Manual backup
docker exec rmk-dgraph-alpha dgraph backup --backup-dir /dgraph/backups
```

Configure automated backups in `.env.oracle`:

```bash
DGRAPH_BACKUP_SCHEDULE=0 2 * * *  # Daily at 2 AM
BACKUP_RETENTION_DAYS=30
```

**Block Volume Snapshots:**

Use OCI Console to schedule block volume snapshots.

### 4. Updates

```bash
# Pull latest changes
cd /opt/rmk
git pull

# Rebuild and restart
docker-compose -f docker-compose.oracle.yml build
docker-compose -f docker-compose.oracle.yml up -d
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
docker-compose -f docker-compose.oracle.yml logs <service-name>

# Check resource usage
docker stats
```

### Connection Refused

- Verify security list rules allow traffic
- Check firewall status: `sudo iptables -L`
- Verify service is listening: `sudo netstat -tlnp`

### NIM API Errors

- Verify API key is valid: check [build.nvidia.com](https://build.nvidia.com)
- Check for rate limits (NIM has rate limits based on tier)
- Ensure `NIM_BASE_URL` is correct in `.env`

## Support

For issues or questions:
- GitHub Issues: <your-repo-url>/issues
- Documentation: <your-docs-url>
