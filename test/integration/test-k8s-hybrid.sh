#!/bin/bash
# ==============================================================================
# E2E Kubernetes Hybrid Test for ldap-es-syncer (on Kind)
# ==============================================================================
set -e

# Move to project root
cd "$(dirname "$0")/../.."

RELEASE_NAME="ldap-es-syncer-test"
JOB_NAME="hybrid-test-job"
IMAGE_NAME="ldap-es-syncer"
IMAGE_TAG="latest"
GATEWAY_IP="172.18.0.1"

echo "==> [1/6] Building local Docker image..."
docker build -f build/package/Dockerfile -t ${IMAGE_NAME}:${IMAGE_TAG} .

# Note: Since Docker Desktop's Kubernetes shares the same containerd/Docker runtime backend,
# we do not need to run "kind load" to copy the image, it is immediately available locally.

echo "==> [2/6] Clean up any old resources..."
docker run --rm \
  -v $(pwd):/apps \
  -v ${HOME}/.kube:/root/.kube \
  --network host \
  alpine/helm uninstall ${RELEASE_NAME} || true
kubectl delete job ${JOB_NAME} --ignore-not-found || true

echo "==> [3/6] Deploying Helm Chart in batch mode (daemonMode=false)..."
# We map hostAliases to redirect 'ldap' and 'elasticsearch' to the Docker host's Kind gateway IP (172.18.0.1)
docker run --rm \
  -v $(pwd):/apps \
  -v ${HOME}/.kube:/root/.kube \
  --network host \
  alpine/helm install ${RELEASE_NAME} /apps/deploy/helm/ldap-es-syncer \
    --set sync.daemonMode=false \
    --set image.repository=${IMAGE_NAME} \
    --set image.tag=${IMAGE_TAG} \
    --set image.pullPolicy=IfNotPresent \
    --set ldap.url="ldap://ldap:389" \
    --set elasticsearch.addresses="http://elasticsearch:9200" \
    --set hostAliases[0].ip="${GATEWAY_IP}" \
    --set hostAliases[0].hostnames[0]="ldap" \
    --set hostAliases[0].hostnames[1]="elasticsearch" \
    --wait

echo "==> [4/6] Creating test Job manually from CronJob..."
# In Helm, daemonMode=false creates a CronJob. We trigger a manual Job run from it.
kubectl create job --from=cronjob/${RELEASE_NAME} ${JOB_NAME}

echo "==> [5/6] Waiting for the Job to finish and asserting logs..."
# Wait for the job pod to start and finish
sleep 5
POD_NAME=$(kubectl get pods -l job-name=${JOB_NAME} -o jsonpath='{.items[0].metadata.name}' || echo "")

if [ -z "${POD_NAME}" ]; then
  echo "Error: No pod found for job ${JOB_NAME}"
  exit 1
fi

echo "Waiting for pod ${POD_NAME} to complete..."
kubectl wait --for=condition=ready pod/${POD_NAME} --timeout=30s || true
# Let it run
sleep 5

POD_LOGS=$(kubectl logs pod/${POD_NAME})
echo "--------------------------------------------------"
echo "${POD_LOGS}"
echo "--------------------------------------------------"

# Asserting that the sync logs contain the expected success statistics
if echo "${POD_LOGS}" | grep -q "User synchronization process completed"; then
  echo "==> Success: Synchronization E2E Job completed successfully inside Kind!"
  SUCCESS=true
else
  echo "==> Error: Synchronization check failed inside the pod logs."
  if echo "${POD_LOGS}" | grep -E -q "connection refused|dial tcp|timed out|context deadline exceeded"; then
    echo "----------------------------------------------------------------------"
    echo "  [TIP] ローカルの Docker Compose サービスが起動していない可能性があります。"
    echo "  テストを実行する前に、以下のコマンドを実行してミドルウェア環境を準備してください："
    echo "      docker compose up -d"
    echo "----------------------------------------------------------------------"
  fi
  SUCCESS=false
fi

echo "==> [6/6] Cleaning up resources..."
docker run --rm \
  -v $(pwd):/apps \
  -v ${HOME}/.kube:/root/.kube \
  --network host \
  alpine/helm uninstall ${RELEASE_NAME}

kubectl delete job ${JOB_NAME} --ignore-not-found

if [ "${SUCCESS}" = "true" ]; then
  echo "==> Hybrid K8s Integration Test Passed!"
  exit 0
else
  echo "==> Hybrid K8s Integration Test Failed!"
  exit 1
fi
