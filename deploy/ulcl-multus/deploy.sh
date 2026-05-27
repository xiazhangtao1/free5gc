#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

NAMESPACE=${NAMESPACE:-free5gc}
RELEASE=${RELEASE:-free5gc}
CHART=${CHART:-"$ROOT_DIR/deploy/free5gc-helm/charts/free5gc"}
VALUES=${VALUES:-"$SCRIPT_DIR/values.yaml"}
SEED_DEFAULT_SUBSCRIBER=${SEED_DEFAULT_SUBSCRIBER:-true}

if [[ ! -d "$CHART" ]]; then
  echo "Helm chart not found: $CHART" >&2
  echo "Run: git submodule update --init --recursive deploy/free5gc-helm" >&2
  exit 1
fi

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install "$RELEASE" "$CHART" \
  --namespace "$NAMESPACE" \
  --values "$VALUES" \
  --wait \
  --timeout "${HELM_TIMEOUT:-15m}"

kubectl get pods -n "$NAMESPACE" -o wide

if [[ "$SEED_DEFAULT_SUBSCRIBER" == "true" ]]; then
  "$SCRIPT_DIR/seed-subscriber.sh"
fi
