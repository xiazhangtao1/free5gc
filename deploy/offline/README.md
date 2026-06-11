# Offline build cache

Use this directory to make the ULCL free5GC build usable without internet
access.

Online preparation:

```bash
./deploy/offline/prepare-offline-cache.sh
NFS=pcf IMPORT_TO_CONTAINERD=true OFFLINE_BUILD=auto DOCKER_BUILD_ARGS=--network=host ./deploy/ulcl-multus/build-images.sh
./deploy/offline/save-images.sh
```

Offline build:

```bash
OFFLINE_BUILD=true ./deploy/ulcl-multus/build-images.sh
```

Offline image loading:

```bash
IMPORT_TO_CONTAINERD=true ./deploy/offline/load-images.sh
```

`OFFLINE_BUILD=false` forces the original online behavior. `OFFLINE_BUILD=auto`
uses local cache when it is present and otherwise keeps the online behavior.
