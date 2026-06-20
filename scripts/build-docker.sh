#!/bin/bash
# ==============================================================================
# Docker Image Build Script for ldap-es-syncer
# ==============================================================================
set -e

IMAGE_NAME="ldap-es-syncer"
TAG="latest"
REGISTRY="localhost:5001"

# Move to project root directory
cd "$(dirname "$0")/.."

echo "==> Building Docker image: ${IMAGE_NAME}:${TAG}..."
docker build -f build/package/Dockerfile -t ${IMAGE_NAME}:${TAG} .

echo ""
echo "==> Build complete!"
echo "Image Tag: ${IMAGE_NAME}:${TAG}"
echo ""
echo "==> To push to local Docker Registry:"
echo "    docker tag ${IMAGE_NAME}:${TAG} ${REGISTRY}/${IMAGE_NAME}:${TAG}"
echo "    docker push ${REGISTRY}/${IMAGE_NAME}:${TAG}"
echo ""
echo "==> To load into Kind Cluster (local Kubernetes testing):"
echo "    kind load docker-image ${IMAGE_NAME}:${TAG}"
echo ""
