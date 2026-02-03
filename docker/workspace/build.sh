#!/bin/bash
# Build and push workspace images

set -e

REGISTRY="ghcr.io/terra-clan/sandbox-engine"
VERSION="${1:-latest}"

echo "Building workspace images..."

# Build base image first
echo "Building workspace-base:${VERSION}..."
docker build -f Dockerfile.base -t ${REGISTRY}/workspace-base:${VERSION} .

# Build language-specific images
echo "Building workspace-node:${VERSION}..."
docker build -f Dockerfile.node -t ${REGISTRY}/workspace-node:${VERSION} .

echo "Building workspace-go:${VERSION}..."
docker build -f Dockerfile.go -t ${REGISTRY}/workspace-go:${VERSION} .

echo "Building workspace-python:${VERSION}..."
docker build -f Dockerfile.python -t ${REGISTRY}/workspace-python:${VERSION} .

echo "All images built successfully!"

# Push if --push flag is provided
if [ "$2" == "--push" ]; then
    echo "Pushing images to registry..."
    docker push ${REGISTRY}/workspace-base:${VERSION}
    docker push ${REGISTRY}/workspace-node:${VERSION}
    docker push ${REGISTRY}/workspace-go:${VERSION}
    docker push ${REGISTRY}/workspace-python:${VERSION}
    echo "All images pushed!"
fi
