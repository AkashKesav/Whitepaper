#!/bin/bash
# VPS Deployment Script - Deploy to any Linux VPS with Docker
# Usage: ./deploy.sh [--pull] [--build]

set -e

echo "🚀 Reflective Memory Kernel - VPS Deployment"
echo "============================================="

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Installing..."
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker $USER
    echo "✅ Docker installed. Please log out and back in, then re-run this script."
    exit 0
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose not found. Installing..."
    sudo apt-get update && sudo apt-get install -y docker-compose-plugin
fi

# Parse arguments
PULL=false
BUILD=false
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --pull) PULL=true ;;
        --build) BUILD=true ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

# Create .env if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env file from template..."
    cp .env.example .env
    # Generate random JWT secret
    JWT_SECRET=$(openssl rand -base64 32)
    sed -i "s/JWT_SECRET=.*/JWT_SECRET=$JWT_SECRET/" .env
    echo "⚠️  Please edit .env and add your API keys (OPENAI_API_KEY, etc.)"
fi

# Pull latest if requested
if [ "$PULL" = true ]; then
    echo "📥 Pulling latest changes..."
    git pull origin main
fi

# Build or pull images
if [ "$BUILD" = true ]; then
    echo "🔨 Building Docker images..."
    docker-compose build
else
    echo "📦 Pulling Docker images..."
    docker-compose pull || true
fi

# Start services
echo "🚀 Starting services..."
docker-compose up -d

# Wait for health
echo "⏳ Waiting for services to be healthy..."
sleep 30

# Check health
echo ""
echo "📊 Service Status:"
docker-compose ps

# Get host IP
HOST_IP=$(hostname -I | awk '{print $1}')

echo ""
echo "============================================="
echo "✅ Deployment Complete!"
echo ""
echo "🌐 Access your app at:"
echo "   http://$HOST_IP:8080"
echo ""
echo "📋 Useful commands:"
echo "   docker-compose logs -f          # View logs"
echo "   docker-compose restart monolith # Restart app"
echo "   docker-compose down             # Stop all"
echo "   docker-compose pull && docker-compose up -d  # Update"
echo "============================================="
