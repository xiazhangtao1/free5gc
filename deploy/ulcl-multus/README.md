# free5GC UL-CL Multus Helm Deployment

This directory contains the project overlay for deploying free5GC on Kubernetes
with UL-CL enabled.

The required Helm chart is vendored in `deploy/free5gc-helm`; deployment does
not require `git submodule update` or network access to fetch chart files.

## Topology

- `BranchingUPF` (`iupf1`): N3-facing intermediate UPF and UL classifier.
- `AnchorUPF1` (`psaupf1`): default anchor UPF for `10.60.0.0/17`.
- `AnchorUPF2` (`psaupf2`): edge anchor UPF for `10.60.128.0/17` and selected destination traffic.
- `AMF`: exposes N2 through NodePort for external gNBs on the same reachable node network.

Default N6 layout in this overlay:

- Center DN: `psaupf1 n6 = 10.100.100.12/24`, gateway `10.100.100.1`.
- Edge DN: `psaupf2 n6 = 10.100.200.12/24`, gateway `10.100.200.1`.
- UE address pools: `10.60.0.0/17` for `AnchorUPF1`, `10.60.128.0/17` for `AnchorUPF2`.

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
- `global.amf.service.ngap.nodeport`: external SCTP NodePort for NGAP. Defaults to `31412`.
- `global.upf.service.gtpu.advertiseAddress`: Kubernetes node IP advertised to gNBs for N3 GTP-U.
- `free5gc-upf.iupf1.service.gtpu.nodePort`: diagnostic UDP NodePort for N3 GTP-U. Defaults to `32152`; the Service port remains `2152/UDP`.
- `free5gc-upf.iupf1.service.gtpu.hostPort`: host UDP port for standards-compliant external gNB N3 traffic. Defaults to `2152` in this overlay because gNBs normally send GTP-U to UDP/2152.
- `global.smf.multus.n4network` and `global.upf.multus.n4network`: PFCP network.
- `global.upf.multus.n6network`: default DN network, still used by `iupf1`.
- `global.upf.multus.n6network.psaupf1`: center DN N6 NAD for `psaupf1`.
- `global.upf.multus.n6network.psaupf2`: edge DN N6 NAD for `psaupf2`.
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
- UE center subnet: `10.60.0.0/17`
- UE edge subnet: `10.60.128.0/17`
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

External-router return routes for the default lab layout:

```text
10.60.0.0/17     via 10.100.100.12
10.60.128.0/17   via 10.100.200.12
```

If the external DN performs NAT itself, host iptables NAT is not required.

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

```bash
./deploy/ulcl-multus/build-images.sh
./deploy/ulcl-multus/deploy.sh
```

The image build uses `docker/free5gc/Dockerfile`, whose builder and runtime
stages are based on Ubuntu 22.04. If the Kubernetes node uses containerd
directly, import the images into the `k8s.io` namespace:

```bash
IMPORT_TO_CONTAINERD=true ./deploy/ulcl-multus/build-images.sh
```

Useful overrides:

```bash
NAMESPACE=free5gc RELEASE=free5gc HELM_TIMEOUT=20m ./deploy/ulcl-multus/deploy.sh
```

To run only the host N6 network setup:

```bash
./deploy/ulcl-multus/setup-n6-host.sh
```

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
- gNB completes NG Setup against `<node-ip>:31412/SCTP`.
- gNB sends N3 GTP-U to `<node-ip>:2152/UDP`. The `32152/UDP` NodePort is also present for diagnostics or non-standard clients that can target a custom GTP-U port.
- UE registration and PDU session establishment succeed.
- Traffic to destinations in `ueRoutingInfo.specificPath` goes through the edge anchor UPF.
- UE business traffic can ping through the PDU tunnel, for example:

```bash
kubectl exec -n default deploy/nrue-oai-nr-ue -- ping -I oaitun_ue1 -c 3 8.8.8.8
```

## Troubleshooting Notes

- If UE registration and PDU Session succeed but external ping fails, first
  confirm host N6 gateway interfaces, return routes, and NAT/FORWARD rules.
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
