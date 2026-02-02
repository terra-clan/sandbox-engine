#!/bin/bash
set -e

REGISTRY=${REGISTRY:-"ghcr.io/terra-clan/sandbox-engine"}

echo "Building code-server base image..."
docker build -t $REGISTRY/code-server:latest ./code-server

echo "Building code-server-python..."
docker build -t $REGISTRY/code-server-python:latest ./code-server-python

echo "Building code-server-node..."
docker build -t $REGISTRY/code-server-node:latest ./code-server-node

echo "Building code-server-go..."
docker build -t $REGISTRY/code-server-go:latest ./code-server-go

echo "Done!"
