# Oracle Cloud Deployment Guide

## Prerequisites

1. **Oracle Compute Instance** (Recommended: VM.Standard.E4.Flex)
   - 4+ OCPU
   - 16GB+ RAM
   - Ubuntu 22.04 or Oracle Linux

2. **Block Volumes** (for data persistence)
   - 50GB+ for DGraph data
   - 20GB+ for Ollama models
   - 10GB+ for Qdrant

## Step 1: Prepare Oracle Instance

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

## Step 2: Setup Block Volumes (Optional but Recommended)

```bash
# Create mount points
sudo mkdir -p /mnt/block_storage/{dgraph,redis,ollama,qdrant}

# Mount your block volumes here (via Oracle Cloud Console or fstab)
# Example for /dev/oracleoci/oraclevdb (adjust path as needed)
# sudo mkfs.ext4 /dev/oracleoci/oraclevdb
# sudo mount /dev/oracleoci/oraclevdb /mnt/block_storage
```

## Step 3: Deploy Application

```bash
# Clone your repository
git clone <your-repo-url>
cd <your-repo>

# Create production environment file
cp .env.production.template .env.production
nano .env.production  # Edit with your values

# Generate secure JWT secret
openssl rand -base64 32  # Use this as JWT_SECRET

# Set your domain
# ALLOWED_ORIGINS=https://your-domain.com

# Deploy
chmod +x deploy-oracle.sh
./deploy-oracle.sh
```

## Step 4: Configure Oracle Security Lists / Firewall

Open these ports in your Oracle Cloud VCN Security List:

| Port | Protocol | Source | Description |
|------|----------|--------|-------------|
| 9090 | TCP | 0.0.0.0/0 | Main Application |
| 443 | TCP | 0.0.0.0/0 | HTTPS (if using SSL) |

## Step 5: Setup SSL/TLS (Recommended)

Option A: Use Oracle Load Balancer with SSL certificate

Option B: Use Nginx reverse proxy with Let's Encrypt:

```bash
# Install Nginx
sudo apt install nginx certbot python3-certbot-nginx -y

# Configure Nginx reverse proxy
sudo nano /etc/nginx/sites-available/rmk
```

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    location / {
        proxy_pass http://localhost:9090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
# Enable site and get certificate
sudo ln -s /etc/nginx/sites-available/rmk /etc/nginx/sites-enabled/
sudo certbot --nginx -d your-domain.com
sudo systemctl restart nginx
```

## Step 6: Verify Deployment

```bash
# Check all services are running
docker compose -f docker-compose.prod.yml ps

# Check logs
docker compose -f docker-compose.prod.yml logs -f monolith

# Test health endpoint
curl http://localhost:9090/health
```

## Step 7: Initial Setup

1. Open https://your-domain.com in browser
2. Register an admin account
3. Go to Settings → AI Providers
4. Add your API keys (NVIDIA NIM, OpenAI, etc.)

## Monitoring Commands

```bash
# View all logs
docker compose -f docker-compose.prod.yml logs -f

# View specific service logs
docker compose -f docker-compose.prod.yml logs -f monolith
docker compose -f docker-compose.prod.yml logs -f ai-services

# Restart services
docker compose -f docker-compose.prod.yml restart

# Stop everything
docker compose -f docker-compose.prod.yml down

# Update and redeploy
git pull
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

## Troubleshooting

### Services not starting
```bash
# Check disk space
df -h

# Check Docker logs
sudo journalctl -u docker -n 50

# Check service health
docker compose -f docker-compose.prod.yml ps
```

### Permission issues
```bash
# Ensure proper ownership
sudo chown -R $USER:$USER /mnt/block_storage
```

## Resource Recommendations

| Component | Min CPU | Min RAM | Recommended CPU | Recommended RAM |
|-----------|---------|---------|-----------------|-----------------|
| Monolith | 1 | 1GB | 2-4 | 2-4GB |
| AI Services | 0.5 | 256MB | 1-2 | 1-2GB |
| DGraph Alpha | 0.5 | 512MB | 1-2 | 2-4GB |
| Ollama | 1 | 2GB | 2-4 | 4-8GB |
| Redis | 0.25 | 128MB | 0.5 | 512MB |
| Qdrant | 0.25 | 128MB | 0.5-1 | 512MB-1GB |
| **Total** | **4** | **6GB** | **8-12** | **12-20GB** |
