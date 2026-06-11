# Offline build cache

Use this directory to make the ULCL free5GC build usable without internet
access.

## What is cached

- `cache/go`: Go toolchain tarball used by `docker/free5gc/Dockerfile`.
- `cache/gomod`: Go module download cache used with `GOPROXY=off`.
- `cache/apt/builder`: local apt repository for the Docker builder stage.
- `cache/apt/runtime`: local apt repository for the Docker runtime stage.
- `images`: optional local directory for saved Docker image archives.

## Online preparation

Run this on a machine that can access Go, Ubuntu apt repositories, Go module
proxies, and image registries:

```bash
./deploy/offline/prepare-offline-cache.sh
```

Build the local free5GC images while online:

```bash
DOCKER_BUILD_ARGS=--network=host IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

Save runtime images for transfer to an offline host:

```bash
./deploy/offline/save-images.sh
```

The image list is maintained in `images.txt`. Add gNB/UE images there if the
offline environment also needs to deploy those charts.

## Offline build

Use `OFFLINE_BUILD=true` to fail fast if any required cache is missing:

```bash
OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none ./deploy/ulcl-multus/build-images.sh
```

For a single NF test:

```bash
NFS=pcf OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none ./deploy/ulcl-multus/build-images.sh
```

If the Kubernetes node uses containerd directly:

```bash
OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

## Offline image loading

Move `images/free5gc-offline-images.tar` to the offline host, then run:

```bash
IMPORT_TO_CONTAINERD=true ./deploy/offline/load-images.sh
```

If Docker is the only runtime, omit `IMPORT_TO_CONTAINERD=true`.

## Build modes

`OFFLINE_BUILD=false` forces the original online behavior. `OFFLINE_BUILD=auto`
uses local cache when it is present and otherwise keeps the online behavior.
`OFFLINE_BUILD=true` requires local cache and blocks online fallback.
