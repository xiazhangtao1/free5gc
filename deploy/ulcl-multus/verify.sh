#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

NAMESPACE=${NAMESPACE:-free5gc}
RELEASE=${RELEASE:-free5gc}
CHART=${CHART:-"$ROOT_DIR/deploy/free5gc-helm/charts/free5gc"}
VALUES=${VALUES:-"$SCRIPT_DIR/values.yaml"}
RENDERED=${RENDERED:-"/tmp/free5gc-ulcl-multus-rendered.yaml"}

helm lint "$CHART" --values "$VALUES"
helm template "$RELEASE" "$CHART" --namespace "$NAMESPACE" --values "$VALUES" > "$RENDERED"

grep -q "kind: NetworkAttachmentDefinition" "$RENDERED"
grep -q "n2network" "$RENDERED"
grep -q "n3network" "$RENDERED"
grep -q "n4network" "$RENDERED"
grep -q "n6network" "$RENDERED"
grep -q "n9network" "$RENDERED"
grep -q "userPlaneArchitecture: ulcl" "$VALUES"
grep -q "BranchingUPF" "$RENDERED"
grep -q "AnchorUPF1" "$RENDERED"
grep -q "AnchorUPF2" "$RENDERED"

if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl get pods -n "$NAMESPACE" -o wide || true
  kubectl get network-attachment-definitions -n "$NAMESPACE" || true
fi

echo "UL-CL Helm overlay verification passed: $RENDERED"
