#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$SCRIPT_DIR/cache}"
GO_VERSION="${GO_VERSION:-1.25.5}"
NFS="${NFS:-amf ausf chf nef nrf nssf pcf smf udm udr upf}"

GO_TARBALL="$CACHE_DIR/go/go${GO_VERSION}.linux-amd64.tar.gz"
GOMODCACHE="$CACHE_DIR/gomod/pkg/mod"

download_go() {
  mkdir -p "$(dirname "$GO_TARBALL")"
  if [ ! -s "$GO_TARBALL" ]; then
    curl -fL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "$GO_TARBALL"
  fi
}

ensure_go() {
  if command -v go >/dev/null 2>&1 && go version | grep -q "go${GO_VERSION} "; then
    command -v go
    return
  fi

  local go_dir="$CACHE_DIR/go/go${GO_VERSION}"
  if [ ! -x "$go_dir/bin/go" ]; then
    rm -rf "$go_dir"
    mkdir -p "$go_dir"
    tar -C "$go_dir" --strip-components=1 -xzf "$GO_TARBALL"
  fi
  printf '%s\n' "$go_dir/bin/go"
}

download_go_modules() {
  local go_bin="$1"
  mkdir -p "$GOMODCACHE"

  for nf in $NFS; do
    local nf_dir="$ROOT_DIR/NFs/$nf"
    if [ -f "$nf_dir/go.mod" ]; then
      echo "downloading Go modules for NFs/$nf"
      (
        cd "$nf_dir"
        GOMODCACHE="$GOMODCACHE" GOCACHE="$CACHE_DIR/gomod/build-cache" "$go_bin" mod download
      )
    fi
  done
}

dependency_closure() {
  apt-cache depends --recurse --no-recommends --no-suggests --no-conflicts \
    --no-breaks --no-replaces --no-enhances "$@" \
    | awk '/^[[:alnum:]][[:alnum:]+.-]+$/ { print }' \
    | sort -u
}

download_debs() {
  local repo_dir="$1"
  shift
  mkdir -p "$repo_dir"

  (
    cd "$repo_dir"
    dependency_closure "$@" | xargs -r apt-get download
    dpkg-scanpackages . /dev/null | gzip -9c > Packages.gz
  )
}

download_go
go_bin="$(ensure_go)"
download_go_modules "$go_bin"

echo "downloading builder deb cache"
download_debs "$CACHE_DIR/apt/builder" ca-certificates curl git make xz-utils

echo "downloading runtime deb cache"
download_debs "$CACHE_DIR/apt/runtime" ca-certificates iproute2 iptables iperf iperf3 netbase tcpdump tini

echo "offline cache prepared under $CACHE_DIR"
