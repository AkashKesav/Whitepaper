#!/bin/bash
# Reflective Memory Kernel - Oracle Cloud Deployment Script

set -e

echo "=========================================="
echo "RMK - Oracle Cloud Deployment"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if .env.production exists
if [ ! -f .env.production ]; then
    echo -e "${RED}ERROR: .env.production not found!${NC}"
    echo "Create it from the template:"
    echo "  cp .env.production.template .env.production"
    echo "  nano .env.production  # Edit with your values"
    exit 1
fi

# Source the production env file
set -a
source .env.production
set +a

# Validate required variables
if [ -z "$JWT_SECRET" ] || [ "$JWT_SECRET" = "CHANGE_THIS_TO_A_STRONG_RANDOM_SECRET_MIN_32_CHARS" ]; then
    echo -e "${RED}ERROR: JWT_SECRET not set or still using default value!${NC}"
    echo "Generate a secure secret:"
    echo "  openssl rand -base64 32"
    exit 1
fi

if [ ${#JWT_SECRET} -lt 32 ]; then
    echo -e "${RED}ERROR: JWT_SECRET must be at least 32 characters!${NC}"
    exit 1
fi

echo -e "${GREEN}✓ JWT_SECRET is configured${NC}"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}ERROR: Docker not found${NC}"
    exit 1
fi

# Check Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${RED}ERROR: Docker Compose not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker is available${NC}"

# Pull latest images
echo ""
echo "Pulling latest images..."
docker compose -f docker-compose.prod.yml pull

# Build images
echo ""
echo "Building application images..."
docker compose -f docker-compose.prod.yml build

# Start services
echo ""
echo "Starting services..."
docker compose -f docker-compose.prod.yml --env-file .env.production up -d

# Wait for health checks
echo ""
echo "Waiting for services to be healthy..."
sleep 10

# Check service status
echo ""
echo "Service Status:"
docker compose -f docker-compose.prod.yml ps

echo ""
echo -e "${GREEN}=========================================="
echo "Deployment Complete!"
echo "==========================================${NC}"
echo ""
echo "Application should be available at: http://localhost:9090"
echo ""
echo "To view logs:"
echo "  docker compose -f docker-compose.prod.yml logs -f"
echo ""
echo "To stop:"
echo "  docker compose -f docker-compose.prod.yml down"
echo ""
