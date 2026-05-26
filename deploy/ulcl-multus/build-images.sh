#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKERFILE="${DOCKERFILE:-$ROOT_DIR/docker/free5gc/Dockerfile}"
TAG="${TAG:-v4.2.2}"
BUILDER="${BUILDER:-docker}"
IMPORT_TO_CONTAINERD="${IMPORT_TO_CONTAINERD:-false}"
NFS="${NFS:-amf ausf chf nef nrf nssf pcf smf udm udr upf}"
DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS:-}"

if ! command -v "$BUILDER" >/dev/null 2>&1; then
  echo "image builder '$BUILDER' not found" >&2
  exit 1
fi

for nf in $NFS; do
  if [ ! -d "$ROOT_DIR/NFs/$nf/cmd" ]; then
    echo "missing source for NFs/$nf; run: git submodule update --init --recursive" >&2
    exit 1
  fi
done

for nf in $NFS; do
  image="free5gc/$nf:$TAG"
  echo "building $image"
  "$BUILDER" build \
    $DOCKER_BUILD_ARGS \
    -f "$DOCKERFILE" \
    --build-arg "NF=$nf" \
    -t "$image" \
    "$ROOT_DIR"

  if [ "$IMPORT_TO_CONTAINERD" = "true" ]; then
    echo "importing $image into containerd namespace k8s.io"
    "$BUILDER" save "$image" | sudo ctr -n k8s.io images import -
  fi
done
