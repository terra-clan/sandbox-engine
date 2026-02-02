#!/bin/bash
set -e

echo "🚀 Deploying sandbox-engine..."

# Pull latest code
git pull origin main

# Build and start
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# Wait for services
echo "⏳ Waiting for services..."
sleep 10

# Check health
docker compose -f docker-compose.prod.yml ps

echo "✅ Deployment complete!"
echo "API: https://api.terra-sandbox.ru"
echo "Traefik: https://traefik.terra-sandbox.ru"
