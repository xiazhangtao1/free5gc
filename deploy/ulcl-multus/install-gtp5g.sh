#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)
GTP5G_DIR=${GTP5G_DIR:-"$ROOT_DIR/deploy/gtp5g"}

if [[ ! -d "$GTP5G_DIR" ]]; then
  echo "gtp5g source not found: $GTP5G_DIR" >&2
  exit 1
fi

if [[ ! -d "/lib/modules/$(uname -r)/build" ]]; then
  echo "kernel headers are missing for $(uname -r)" >&2
  echo "Install them before running this script." >&2
  exit 1
fi

if ! command -v gcc-12 >/dev/null 2>&1; then
  echo "gcc-12 is required to build gtp5g for this kernel." >&2
  echo "Ubuntu: sudo apt-get install -y gcc-12 g++-12" >&2
  exit 1
fi

make -C "$GTP5G_DIR" clean
make -C "$GTP5G_DIR"
sudo make -C "$GTP5G_DIR" install

if ! lsmod | grep -q '^gtp5g'; then
  echo "gtp5g was installed but is not loaded." >&2
  if command -v mokutil >/dev/null 2>&1 && mokutil --sb-state 2>/dev/null | grep -qi enabled; then
    echo "Secure Boot is enabled. Disable Secure Boot or sign/enroll the module with MOK, then reboot." >&2
  fi
  exit 1
fi

lsmod | grep -E '^(gtp5g|udp_tunnel)'
