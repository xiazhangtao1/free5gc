#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_LIST="${IMAGE_LIST:-$SCRIPT_DIR/images.txt}"
OUT="${OUT:-$SCRIPT_DIR/images/free5gc-offline-images.tar}"

mapfile -t images < <(grep -vE '^\s*(#|$)' "$IMAGE_LIST")

mkdir -p "$(dirname "$OUT")"
docker pull "${images[@]}"
docker save -o "$OUT" "${images[@]}"

echo "saved ${#images[@]} images to $OUT"
