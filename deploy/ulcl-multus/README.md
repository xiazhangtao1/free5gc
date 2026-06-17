# free5GC UL-CL Multus Helm Deployment

This directory contains the project overlay for deploying free5GC on Kubernetes
with UL-CL enabled.

The required Helm chart is vendored in `deploy/free5gc-helm`. NF and webconsole
sources are also vendored into this branch instead of being kept as Git
submodules, so deployment and image builds do not require `git submodule update`
or network access to fetch source repositories.

## Topology

- `BranchingUPF` (`iupf1`): N3-facing intermediate UPF and UL classifier.
- `AnchorUPF1` (`psaupf1`): default anchor UPF for the common UE pool.
- `AnchorUPF2` (`psaupf2`): edge anchor UPF for selected destination traffic.
- `AMF`: exposes N2 through hostPort by default for external gNBs on the same reachable node network.

Default N6 layout in this overlay:

- Center DN: `psaupf1 n6 = 10.100.100.12/24`, gateway `10.100.100.1`.
- Edge DN: `psaupf2 n6 = 10.100.200.12/24`, gateway `10.100.200.1`.
- UE address pool: `10.60.0.0/16`. Both PSA UPFs install this pool locally;
  UL-CL chooses the PSA by SMF path selection and destination-specific
  `ueRoutingInfo`, not by assigning disjoint UE address pools per PSA.

These addresses are deployment defaults for this lab. They are not required by
free5GC itself, but the values in `values.yaml`, the host N6 setup script, and
the external DN/routing environment must agree.

## Required Node Setup

On Ubuntu/MicroK8s, prepare the cluster before installation:

```bash
sudo snap install microk8s --classic --channel=1.28/stable
sudo snap install kubectl --classic
sudo snap install helm --classic
sudo usermod -aG microk8s "$USER"
newgrp microk8s
microk8s enable hostpath-storage
microk8s enable community
microk8s enable multus
```

The UPF pods require `net.ipv4.ip_forward=1` and `NET_ADMIN`. With MicroK8s
and Calico, also allow forwarding in Calico and kubelet unsafe sysctls as
described by the upstream free5gc-helm guide.

This overlay avoids kubelet `allowed-unsafe-sysctls` requirements by writing
`1` to `/proc/sys/net/ipv4/ip_forward` from the UPF wrapper script. If your
cluster policy requires Pod-level sysctls instead, remove the UPF
`podSecurityContext.sysctls: []` overrides and allowlist `net.ipv4.ip_forward`
in kubelet.

## Configure

Edit `values.yaml` before production use:

- `global.amf.multus.n2network.masterIf`: host interface reachable by the gNB N2 network. Defaults to `wlp128s0` for the current host.
- `global.upf.multus.n3network.masterIf`: host interface reachable by the gNB N3 network.
- `global.amf.service.ngap.hostPort`: host SCTP port for standards-compliant external gNB N2 traffic. Defaults to `38412`.
- `global.amf.service.ngap.nodeport`: SCTP NodePort for NGAP when NodePort mode is used. Defaults to `38412`, and requires the Kubernetes NodePort range to include that port.
- `global.upf.service.gtpu.advertiseAddress`: Kubernetes node IP advertised to gNBs for N3 GTP-U.
- `free5gc-upf.iupf1.service.gtpu.nodePort`: UDP NodePort for N3 GTP-U when NodePort mode is used. Defaults to `2152`, and requires the Kubernetes NodePort range to include that port.
- `free5gc-upf.iupf1.service.gtpu.hostPort`: host UDP port for standards-compliant external gNB N3 traffic. Defaults to `2152` in this overlay because gNBs normally send GTP-U to UDP/2152.
- `global.smf.multus.n4network` and `global.upf.multus.n4network`: PFCP network.
- `global.upf.multus.n6network`: default DN network, still used by `iupf1`.
- `global.upf.multus.n6network.psaupf1`: center DN N6 NAD for `psaupf1`.
- `global.upf.multus.n6network.psaupf2`: edge DN N6 NAD for `psaupf2`.
- `free5gc-upf.psaupf1.configmap.wrapperAdditionalCommands` and
  `free5gc-upf.psaupf2.configmap.wrapperAdditionalCommands`: keep PSA UPF
  default routes on N6. Multus can add default routes for N4/N9/eth0 too; if
  N6 is not the selected default route, UE traffic can establish a PDU session
  but fail to reach the DN or internet.
- `global.upf.multus.n9network`: UPF-to-UPF N9 network.
- `free5gc-smf.smf.configuration.ueRoutingInfo`: IMSI groups and destination-specific edge paths.
- `free5gc-webui.webui.service.nodePort`: defaults to `30510` to avoid the
  upstream default `30500` when another free5GC release is still present.
- `mongodb.persistence.storageClass`: defaults to `local-path`; change it to
  `microk8s-hostpath` on a default MicroK8s hostpath-storage setup.
  The overlay disables the upstream fixed-name `free5gc-pv-mongo` local PV so
  the chart can use the cluster storage class normally.

The default PLMN/DNN/S-NSSAI in this overlay is:

- PLMN: `460/11`
- TAC: `000001`
- DNN: `cmnet`
- S-NSSAI: `sst=1`, `sd=010101`
- Subscriber: `imsi-460110000000100`

The deploy script seeds this subscriber into MongoDB after Helm reports the
Pods ready. Align the gNB and UE/SIM profile with these values.

## N2/N3 Exposure Modes

The N2 control plane and N3 user plane can be exposed in three modes. For
standards-compliant gNBs, use `38412/SCTP` for N2 and `2152/UDP` for N3.

Recommended default for this UL-CL overlay is `hostPort`:

- AMF listens on Pod port `38412/SCTP` and is exposed on host port
  `38412/SCTP`.
- I-UPF listens on Pod port `2152/UDP` and is exposed on host port
  `2152/UDP`.
- Multus remains enabled, so AMF/UPF can still use dedicated N2/N3/N4/N6/N9
  interfaces inside the pods.

### hostPort Mode

Use this when gNB/UE are on another server or another cluster and can reach the
Kubernetes node IP directly. This is the default in `values.yaml`.

```yaml
global:
  amf:
    hostNetwork:
      enabled: false
    service:
      ngap:
        enabled: true
        type: ClusterIP
        bindAddress: 0.0.0.0
        port: 38412
        hostPort:
          enabled: true
          hostIP: ""
          port: 38412
  upf:
    service:
      gtpu:
        enabled: true
        type: ClusterIP
        bindAddress: 0.0.0.0
        port: 2152
        nodePort: 2152
        hostPort:
          enabled: true
          hostIP: ""
          port: 2152

free5gc-upf:
  iupf1:
    hostNetwork:
      enabled: false
    service:
      gtpu:
        enabled: true
        type: ClusterIP
        bindAddress: 0.0.0.0
        port: 2152
        nodePort: 2152
        hostPort:
          enabled: true
          hostIP: ""
          port: 2152
```

Configure the gNB to use:

```text
AMF N2: <k8s-node-ip>:38412/SCTP
UPF N3: <k8s-node-ip>:2152/UDP
```

`bindAddress` controls the address the AMF/UPF process listens on inside its
network namespace. Keep it as `0.0.0.0` when traffic may enter through Multus,
hostPort, NodePort, or different node interfaces. Set it to a specific local IP
only when that address exists in the same network namespace as the container.

`hostPort.hostIP` controls the Kubernetes hostPort binding on the node. Empty
means all node addresses. Set it to a specific node IP, for example
`192.168.25.124`, when the host has multiple addresses and the gNB must use only
one of them.

### NodePort Mode

Use this only if the Kubernetes cluster allows the required standard ports in
the NodePort range. Kubernetes defaults to `30000-32767`, so `38412` and `2152`
normally require changing the API server `--service-node-port-range`.

```yaml
global:
  amf:
    service:
      ngap:
        enabled: true
        type: NodePort
        bindAddress: 0.0.0.0
        port: 38412
        nodeport: 38412
        hostPort:
          enabled: false
  upf:
    service:
      gtpu:
        enabled: true
        type: NodePort
        bindAddress: 0.0.0.0
        port: 2152
        nodePort: 2152
        hostPort:
          enabled: false

free5gc-upf:
  iupf1:
    service:
      gtpu:
        enabled: true
        type: NodePort
        bindAddress: 0.0.0.0
        port: 2152
        nodePort: 2152
        hostPort:
          enabled: false
```

If the cluster cannot expose those NodePorts, do not use non-standard ports for
production gNB integration; use `hostPort`, `hostNetwork`, or an external L4
load balancer that preserves `38412/SCTP` and `2152/UDP`.

NodePort does not provide a per-Service `hostIP` field. Restricting NodePort to
specific node addresses is a kube-proxy/node configuration concern, such as
`--nodeport-addresses`, or should be handled by an external load balancer.

### hostNetwork Mode

Use this when the AMF/UPF should bind directly in the node network namespace.
In this mode, do not enable `hostPort`; the container ports are already host
ports.

```yaml
global:
  amf:
    hostNetwork:
      enabled: true
      dnsPolicy: ClusterFirstWithHostNet
    service:
      ngap:
        enabled: false
        bindAddress: 0.0.0.0
        port: 38412
        hostPort:
          enabled: false

free5gc-upf:
  iupf1:
    hostNetwork:
      enabled: true
      dnsPolicy: ClusterFirstWithHostNet
    service:
      gtpu:
        enabled: true
        type: ClusterIP
        bindAddress: 0.0.0.0
        port: 2152
        hostPort:
          enabled: false
```

For UL-CL, `hostPort` is usually safer than `hostNetwork` because the UPFs still
need well-defined N4/N6/N9 interfaces. If you use `hostNetwork` for UPF, make
sure the node network has equivalent N4/N6/N9 interfaces/routes and that only
one UPF instance binds each host port on a node.

For the current lab UL-CL topology, I-UPF hostNetwork mode requires the
Kubernetes node to own the I-UPF N4 and N9 addresses because those interfaces no
longer come from Multus inside the I-UPF pod:

```bash
sudo ip link add n4host-iupf link wlp128s0 type ipvlan mode l2 2>/dev/null || true
sudo ip addr replace 10.100.50.245/29 dev n4host-iupf
sudo ip link set n4host-iupf up

sudo ip link add n9host-iupf link wlp128s0 type ipvlan mode l2 2>/dev/null || true
sudo ip addr replace 10.100.50.227/29 dev n9host-iupf
sudo ip link set n9host-iupf up
```

Adjust `wlp128s0`, `10.100.50.245/29`, and `10.100.50.227/29` for other
servers. If these addresses are missing, I-UPF can bind GTP-U on
`0.0.0.0:2152` but PFCP will fail with `cannot assign requested address` or
`not found NodeID`.

In this UL-CL lab, keep hostNetwork N2/N3 `bindAddress` as `0.0.0.0` unless
there is a clear need to restrict the listener. Binding N2/N3 to the node IP
can start successfully, but it changes the local N3/N9 path assumptions and
must be revalidated with real UE traffic on the target server.

### Validation Status

Current lab runtime validation on 2026-06-08:

- `hostPort`: OAI gNB/nrUE completed registration and PDU Session Resource
  Setup. `oaitun_ue1` was created and ping to `8.8.8.8` succeeded with 0%
  packet loss. This is the default mode.
- `hostNetwork`: OAI gNB/nrUE completed registration and PDU Session Resource
  Setup. `oaitun_ue1` ping to `8.8.8.8` succeeded with 0% packet loss after
  creating the required host-side I-UPF N4/N9 interfaces.
- `NodePort`: applying standards-compliant `38412/SCTP` and `2152/UDP`
  NodePorts was actually tested on the current cluster and rejected by
  Kubernetes because this kube-apiserver uses the default `30000-32767`
  NodePort range. Use NodePort only after the cluster owner explicitly configures
  `--service-node-port-range` to include `2152` and `38412`, or use `hostPort`
  / `hostNetwork` instead.

## Host N6 Networking

`deploy.sh` calls `setup-n6-host.sh` by default before Helm deployment. The
script creates host-side N6 gateway interfaces, enables IPv4 forwarding, adds
UE return routes, and installs NAT/FORWARD rules. This is needed when the DN
gateway is provided by the Kubernetes node itself instead of an external router
or physical DN.

Current lab defaults:

- N6 master interface: `wlp128s0`
- Center host gateway: `n6host1 = 10.100.100.1/24`
- Edge host gateway: `n6host2 = 10.100.200.1/24`
- Center PSA N6 IP: `10.100.100.12`
- Edge PSA N6 IP: `10.100.200.12`
- UE subnet: `10.60.0.0/16`
- Preferred internet egress: `outline-tun1` if present; otherwise the script
  tries the route to `8.8.8.8`, then the default route.

For another server or network, override these values instead of editing the
script:

```bash
N6_MASTER_IF=enp1s0 \
N6_OUT_IF=enp1s0 \
N6_CENTER_GW=10.10.100.1/24 \
N6_EDGE_GW=10.10.200.1/24 \
N6_CENTER_UPF=10.10.100.12 \
N6_EDGE_UPF=10.10.200.12 \
UE_SUBNET=10.60.0.0/16 \
UE_DEFAULT_UPF=10.10.100.12 \
UE_DEFAULT_IF=n6host1 \
N6_CENTER_SUBNET=10.10.100.0/24 \
N6_EDGE_SUBNET=10.10.200.0/24 \
./deploy/ulcl-multus/deploy.sh
```

When changing these values, also update `values.yaml`:

- `global.upf.multus.*.n6if.ipAddress`
- `global.upf.multus.n6network.psaupf1.*`
- `global.upf.multus.n6network.psaupf2.*`
- `free5gc-upf.psaupf1.configuration.dnnList`
- `free5gc-upf.psaupf2.configuration.dnnList`
- Any `ueRoutingInfo.specificPath` destinations used to demonstrate edge
  steering.

If the N6 gateway is an external router or a real DN instead of the host, skip
host N6 setup and configure that router to route back to the UE pools through
the PSA UPF N6 IPs:

```bash
SETUP_N6_HOST=false ./deploy/ulcl-multus/deploy.sh
```

External-router return route for the default lab layout:

```text
10.60.0.0/16     via the active PSA UPF N6 address for the selected DN path
```

For the host-provided internet/DN path in this lab, `setup-n6-host.sh` defaults
that route to `psaupf1` (`10.100.100.12`). Do not split the same UE pool into
`/17` routes on the same host unless SMF address allocation and UPF selection
are also constrained to match that split; otherwise a UE can receive an address
from one half of the pool while the PDU session uses the other PSA.

If the external DN performs NAT itself, host iptables NAT is not required.

The PSA UPF pods must use their N6 interface as the default route for internet
or DN traffic. The overlay does this in `values.yaml` by deleting Multus-created
N4/N9/eth0 default routes and replacing the default route with:

```text
psaupf1: default via 10.100.100.1 dev n6 onlink
psaupf2: default via 10.100.200.1 dev n6 onlink
```

If those gateways change, update both the N6 network values and the PSA UPF
`wrapperAdditionalCommands`.

## Validate Locally

```bash
./deploy/ulcl-multus/verify.sh
```

This runs `helm lint`, renders the chart, and checks that the UL-CL and Multus
objects are present.

## Install gtp5g

The UPF requires the `gtp5g` kernel module on every Kubernetes node that can run
UPF pods. The source is vendored in `deploy/gtp5g` for offline environments.

```bash
sudo apt-get install -y gcc-12 g++-12
./deploy/ulcl-multus/install-gtp5g.sh
lsmod | grep gtp5g
```

If Secure Boot is enabled, unsigned third-party kernel modules are rejected with
`Key was rejected by service`. Disable Secure Boot in BIOS, or sign `gtp5g.ko`
and enroll the signing certificate with MOK, then reboot before deploying
free5GC.

## Deploy

### Online deployment

Use this path when the host can access Ubuntu apt repositories, Go module
proxies, container image registries, and GitHub.

```bash
./deploy/ulcl-multus/build-images.sh
./deploy/ulcl-multus/deploy.sh
```

The image build uses `docker/free5gc/Dockerfile`, whose builder and runtime
stages are based on Ubuntu 22.04. The default `OFFLINE_BUILD=auto` uses the
local cache under `deploy/offline/cache` when it exists; otherwise it keeps the
online behavior.

If the Kubernetes node uses containerd directly, import the images into the
`k8s.io` namespace:

```bash
IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

If Docker build cannot resolve apt or Go hosts because the host is using a VPN,
use host networking for the image build:

```bash
DOCKER_BUILD_ARGS=--network=host IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

Useful overrides:

```bash
NAMESPACE=free5gc RELEASE=free5gc HELM_TIMEOUT=20m ./deploy/ulcl-multus/deploy.sh
```

To run only the host N6 network setup:

```bash
./deploy/ulcl-multus/setup-n6-host.sh
```

### Offline deployment

The repository includes a pre-populated offline build cache under
`deploy/offline/cache`:

- Go toolchain tarball: `deploy/offline/cache/go`
- Go module download cache: `deploy/offline/cache/gomod`
- Local apt repositories for the Docker builder/runtime stages:
  `deploy/offline/cache/apt`

In an offline environment, build images with networking disabled to verify that
no online downloads are used:

```bash
NFS=pcf OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none ./deploy/ulcl-multus/build-images.sh
```

For a full CN image build:

```bash
OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none ./deploy/ulcl-multus/build-images.sh
```

If the Kubernetes node uses containerd directly:

```bash
OFFLINE_BUILD=true DOCKER_BUILD_ARGS=--network=none IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

Runtime container images must already exist in Docker/containerd in the offline
environment. On an online machine, save commonly used images:

```bash
./deploy/offline/save-images.sh
```

Move `deploy/offline/images/free5gc-offline-images.tar` to the offline machine,
then load it:

```bash
IMPORT_TO_CONTAINERD=true ./deploy/offline/load-images.sh
```

Then deploy normally:

```bash
./deploy/ulcl-multus/deploy.sh
```

To refresh the offline cache while online:

```bash
./deploy/offline/prepare-offline-cache.sh
```

Offline build mode flags:

- `OFFLINE_BUILD=auto`: default. Use local cache when present, otherwise use
  online downloads.
- `OFFLINE_BUILD=true`: require local cache and fail immediately if something
  is missing.
- `OFFLINE_BUILD=false`: force the original online behavior.

## Runtime Checks

```bash
kubectl get pods -n free5gc -o wide
kubectl get network-attachment-definitions -n free5gc
kubectl logs -n free5gc deploy/free5gc-free5gc-smf-smf | grep -i pfcp
kubectl exec -n free5gc deploy/free5gc-free5gc-upf-iupf1 -- cat /proc/sys/net/ipv4/ip_forward
ip -br addr show n6host1
ip -br addr show n6host2
```

Expected results:

- All pods are `Running`.
- SMF logs show PFCP association with all UPFs.
- `ip_forward` returns `1` in UPF pods.
- `n6host1` has `10.100.100.1/24` and `n6host2` has `10.100.200.1/24`
  when host N6 setup is enabled.
- gNB completes NG Setup against `<node-ip>:38412/SCTP`.
- gNB sends N3 GTP-U to `<node-ip>:2152/UDP`.
- UE registration and PDU session establishment succeed.
- Traffic to destinations in `ueRoutingInfo.specificPath` goes through the edge anchor UPF.
- UE business traffic can ping through the PDU tunnel, for example:

```bash
kubectl exec -n default deploy/nrue-oai-nr-ue -- ping -I oaitun_ue1 -c 3 8.8.8.8
```

## XCN Dedicated Bearer API

This branch exposes a PCF-side external API for compute-center driven
dedicated bearer creation and deletion.

The Helm overlay exposes PCF as `NodePort 30777`:

```bash
kubectl get svc free5gc-free5gc-pcf-service -n free5gc -o wide
```

Use `http://<node-ip>:30777/xcn-dedicated-bearer/v1/bearers` from outside the
cluster. The request can identify the target session in these ways:

- `ueIp`: match the existing SM Policy by UE IPv4/IPv6 address.
- `ngapId`: match AMF UE NGAP ID first, then RAN UE NGAP ID.
- `amfUeNgapId` or `ranUeNgapId`: match the explicit NGAP ID type.
- `supi` plus `pduSessionId`: match the existing SM Policy directly.

If multiple selectors are present, target selection priority is
`ueIp` -> `ngapId/amfUeNgapId/ranUeNgapId` -> `supi+pduSessionId`.

Create by `supi + pduSessionId`:

```bash
curl --http2-prior-knowledge -sS -i \
  -X POST http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers \
  -H 'Content-Type: application/json' \
  -d '{
    "supi": "imsi-460110000000100",
    "pduSessionId": 1,
    "mediaType": "audio",
    "flowDescriptions": [
      "permit out ip from any to assigned",
      "permit in ip from assigned to any"
    ],
    "qos": {
      "5qi": 2,
      "arp": {
        "priorityLevel": 8,
        "preemptionCapability": "NOT_PREEMPT",
        "preemptionVulnerability": "PREEMPTABLE"
      },
      "maxbrDl": "10 Mbps",
      "maxbrUl": "10 Mbps",
      "gbrDl": "5 Mbps",
      "gbrUl": "5 Mbps"
    }
  }'
```

Create by `ueIp`:

```bash
curl --http2-prior-knowledge -sS -i \
  -X POST http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers \
  -H 'Content-Type: application/json' \
  -d '{
    "ueIp": "10.60.0.3",
    "supi": "ignored-when-ueIp-is-present",
    "pduSessionId": 99,
    "mediaType": "audio",
    "flowDescriptions": [
      "permit out ip from any to assigned",
      "permit in ip from assigned to any"
    ],
    "qos": {
      "5qi": 2,
      "arp": {
        "priorityLevel": 8,
        "preemptionCapability": "NOT_PREEMPT",
        "preemptionVulnerability": "PREEMPTABLE"
      },
      "maxbrDl": "10 Mbps",
      "maxbrUl": "10 Mbps",
      "gbrDl": "5 Mbps",
      "gbrUl": "5 Mbps"
    }
  }'
```

Create by `ngapId`:

```bash
curl --http2-prior-knowledge -sS -i \
  -X POST http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers \
  -H 'Content-Type: application/json' \
  -d '{
    "ngapId": 1,
    "mediaType": "audio",
    "flowDescriptions": [
      "permit out ip from any to assigned",
      "permit in ip from assigned to any"
    ],
    "qos": {
      "5qi": 2,
      "arp": {
        "priorityLevel": 8,
        "preemptionCapability": "NOT_PREEMPT",
        "preemptionVulnerability": "PREEMPTABLE"
      },
      "maxbrDl": "10 Mbps",
      "maxbrUl": "10 Mbps",
      "gbrDl": "5 Mbps",
      "gbrUl": "5 Mbps"
    }
  }'
```

A successful creation returns `201` and an `appSessionId`, for example:

```json
{"appSessionId":"imsi-460110000000100-1","pccRuleIds":{"1-1":"PccRuleId-2"}}
```

`PccRuleId-1` is the default session rule. XCN dedicated bearer creation uses a
separate PCC/QoS rule, so the returned PCC rule should not be `PccRuleId-1`.

Delete by `appSessionId`:

```bash
curl --http2-prior-knowledge -sS -i \
  -X DELETE http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers/imsi-460110000000100-1
```

Delete by request body:

```bash
curl --http2-prior-knowledge -sS -i \
  -X POST http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers/delete \
  -H 'Content-Type: application/json' \
  -d '{"ueIp":"10.60.0.3"}'
```

Query XCN-created dedicated bearers for a target session:

```bash
curl --http2-prior-knowledge -sS -i \
  'http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers?ueIp=10.60.0.3'

curl --http2-prior-knowledge -sS -i \
  'http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers?ngapId=1'

curl --http2-prior-knowledge -sS -i \
  'http://192.168.25.124:30777/xcn-dedicated-bearer/v1/bearers?supi=imsi-460110000000100&pduSessionId=1'
```

Expected logs after creation:

```bash
kubectl logs -n free5gc -l nf=pcf --since=2m | grep 'SM Policy Update'
kubectl logs -n free5gc -l nf=smf --since=2m | grep 'PFCP Session Modification'
kubectl logs deploy/gnb-oai-gnb --since=2m | grep 'created new DRB 2 for QFI 2'
kubectl logs deploy/nrue-oai-nr-ue --since=2m | grep 'Added DRB 2'
```

Expected logs after deletion:

```bash
kubectl logs -n free5gc -l nf=smf --since=2m | grep 'Remove PCCRule'
kubectl logs -n free5gc -l nf=smf --since=2m | grep 'PFCP Session Modification'
kubectl logs -n free5gc -l nf=amf --since=2m | grep 'Send PDU Session Resource Modify Request'
```

Deletion follows the 5G PDU Session Modification procedure: SMF removes the XCN
PCC/PFCP state and sends a UE-side modification containing NAS QoS Rule/QoS Flow
Description deletion plus NGAP `QosFlowToReleaseList`. In the current OAI gNB
build used for local validation, this terminal-side release can restart the gNB
in `nr_rrc_remove_drb_by_id(... drb_id=0)`. Treat that as an OAI gNB issue; the
core network keeps the standard release signaling enabled.

If the API returns `sm policy not found` after restarting PCF, restart or
reconnect the UE so SMF recreates the SM Policy context in PCF, then retry the
request.

## Troubleshooting Notes

- If UE registration and PDU Session succeed but external ping fails, first
  confirm host N6 gateway interfaces, return routes, NAT/FORWARD rules, and the
  PSA UPF pod default route. `ip route` inside `psaupf1` should prefer
  `default via 10.100.100.1 dev n6`; `psaupf2` should prefer
  `default via 10.100.200.1 dev n6`.
- If UPF/SMF/UE pods are restarted out of order, stale user-plane state can
  produce kernel logs like `gtp5g ... No PDR match this skb : teid[...]`.
  Stop the UE, restart UPFs and SMF, wait for PFCP Association, then start the
  UE again.
- When a host VPN is enabled, the real egress interface may be a tunnel such as
  `outline-tun1`. Set `N6_OUT_IF` explicitly if automatic detection chooses the
  wrong interface.
- `setup-n6-host.sh` uses runtime `ip` and `iptables` state. These settings do
  not survive a host reboot unless the script is run again or the equivalent
  networking is managed by systemd/network-manager/firewall tooling.

## Current Host Notes

- This host uses `wlp128s0`; the overlay defaults all Multus `masterIf` values
  to that interface for local validation.
- CoreDNS can enter `CrashLoopBackOff` while the host VPN is enabled. Keep that
  in mind when waiting for NFs that resolve Kubernetes services such as
  `mongodb` or `nrf-nnrf`.
- Static Multus IPs cannot be reused by two live pods. For failed or interrupted
  installs, clean the namespace before retrying:

```bash
kubectl delete namespace free5gc-ulcl --wait=true --timeout=180s
kubectl delete pv free5gc-pv-mongo 2>/dev/null || true
```

- If a namespace is stuck because an interrupted rollout left old pods holding
  static Multus IPs, inspect with:

```bash
kubectl get pods -n free5gc-ulcl -o wide
kubectl get events -n free5gc-ulcl --sort-by=.lastTimestamp
```
