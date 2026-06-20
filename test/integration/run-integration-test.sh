#!/bin/bash
# ==============================================================================
# Run E2E Integration Tests for ldap-es-syncer
# ==============================================================================
set -e

# Move to project root
cd "$(dirname "$0")/../.."

echo "==> Loading environment variables from .env..."
if [ -f .env ]; then
  # Load .env variables
  set -a
  source .env
  set +a
else
  echo "Error: .env file not found at project root."
  exit 1
fi

echo "==> Running Go E2E Integration Tests..."
go test -v -tags=integration ./test/integration/...

echo "==> Integration tests completed successfully!"
