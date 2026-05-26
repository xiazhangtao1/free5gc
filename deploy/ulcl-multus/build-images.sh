#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKERFILE="${DOCKERFILE:-$ROOT_DIR/docker/free5gc/Dockerfile}"
TAG="${TAG:-v4.2.2}"
BUILDER="${BUILDER:-docker}"
IMPORT_TO_CONTAINERD="${IMPORT_TO_CONTAINERD:-false}"
NFS="${NFS:-amf ausf chf nef nrf nssf pcf smf udm udr upf}"
DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS:-}"
INIT_SUBMODULES="${INIT_SUBMODULES:-false}"

if ! command -v "$BUILDER" >/dev/null 2>&1; then
  echo "image builder '$BUILDER' not found" >&2
  exit 1
fi

echo "NFS=$NFS"
echo "TAG=$TAG"
echo "BUILDER=$BUILDER"
echo "DOCKER_BUILD_ARGS=$DOCKER_BUILD_ARGS"
echo "IMPORT_TO_CONTAINERD=$IMPORT_TO_CONTAINERD"
echo "INIT_SUBMODULES=$INIT_SUBMODULES"

for nf in $NFS; do
  if [ "$nf" = "webui" ] || [ "$nf" = "webconsole" ]; then
    echo "webui is built from the webconsole submodule and is not supported by this NF Dockerfile yet" >&2
    echo "remove webui from NFS, or keep using the upstream free5gc/webui image for now" >&2
    exit 1
  fi

  if [ ! -d "$ROOT_DIR/NFs/$nf/cmd" ]; then
    if [ "$INIT_SUBMODULES" = "true" ]; then
      echo "initializing missing submodule NFs/$nf"
      git -C "$ROOT_DIR" submodule update --init --recursive "NFs/$nf"
    fi

    if [ ! -d "$ROOT_DIR/NFs/$nf/cmd" ]; then
      echo "missing source for NFs/$nf" >&2
      echo "run: git submodule update --init --recursive NFs/$nf" >&2
      echo "or run this script with INIT_SUBMODULES=true" >&2
      exit 1
    fi
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
