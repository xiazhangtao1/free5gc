#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCHIVE="${ARCHIVE:-$SCRIPT_DIR/images/free5gc-offline-images.tar}"
IMPORT_TO_CONTAINERD="${IMPORT_TO_CONTAINERD:-false}"

if [ ! -f "$ARCHIVE" ]; then
  echo "image archive not found: $ARCHIVE" >&2
  exit 1
fi

docker load -i "$ARCHIVE"

if [ "$IMPORT_TO_CONTAINERD" = "true" ]; then
  sudo ctr -n k8s.io images import "$ARCHIVE"
fi
