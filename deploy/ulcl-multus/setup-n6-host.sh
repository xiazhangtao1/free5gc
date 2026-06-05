#!/usr/bin/env bash
set -euo pipefail

N6_MASTER_IF=${N6_MASTER_IF:-wlp128s0}
N6_CENTER_IF=${N6_CENTER_IF:-n6host1}
N6_EDGE_IF=${N6_EDGE_IF:-n6host2}
N6_CENTER_GW=${N6_CENTER_GW:-10.100.100.1/24}
N6_EDGE_GW=${N6_EDGE_GW:-10.100.200.1/24}
N6_CENTER_UPF=${N6_CENTER_UPF:-10.100.100.12}
N6_EDGE_UPF=${N6_EDGE_UPF:-10.100.200.12}
UE_CENTER_SUBNET=${UE_CENTER_SUBNET:-10.60.0.0/17}
UE_EDGE_SUBNET=${UE_EDGE_SUBNET:-10.60.128.0/17}
UE_SUBNET=${UE_SUBNET:-10.60.0.0/16}
N6_CENTER_SUBNET=${N6_CENTER_SUBNET:-10.100.100.0/24}
N6_EDGE_SUBNET=${N6_EDGE_SUBNET:-10.100.200.0/24}
N6_OUT_IF=${N6_OUT_IF:-}

if [[ -z "$N6_OUT_IF" ]] && ip link show outline-tun1 >/dev/null 2>&1; then
  N6_OUT_IF=outline-tun1
fi

if [[ -z "$N6_OUT_IF" ]]; then
  N6_OUT_IF=$(ip route get 8.8.8.8 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i == "dev") {print $(i+1); exit}}' || true)
fi

if [[ -z "$N6_OUT_IF" ]]; then
  N6_OUT_IF=$(ip route show default 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i == "dev") {print $(i+1); exit}}' || true)
fi

sudo ip link show "$N6_CENTER_IF" >/dev/null 2>&1 || sudo ip link add "$N6_CENTER_IF" link "$N6_MASTER_IF" type ipvlan mode l2
sudo ip link show "$N6_EDGE_IF" >/dev/null 2>&1 || sudo ip link add "$N6_EDGE_IF" link "$N6_MASTER_IF" type ipvlan mode l2

sudo ip addr replace "$N6_CENTER_GW" dev "$N6_CENTER_IF"
sudo ip addr replace "$N6_EDGE_GW" dev "$N6_EDGE_IF"
sudo ip link set "$N6_CENTER_IF" up
sudo ip link set "$N6_EDGE_IF" up

sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null

sudo ip route replace "$UE_CENTER_SUBNET" via "$N6_CENTER_UPF" dev "$N6_CENTER_IF"
sudo ip route replace "$UE_EDGE_SUBNET" via "$N6_EDGE_UPF" dev "$N6_EDGE_IF"

if ip route show table 7113 >/dev/null 2>&1; then
  ip rule show | grep -q "from $UE_SUBNET lookup 7113" || sudo ip rule add from "$UE_SUBNET" lookup 7113 priority 1100
fi

if [[ -n "$N6_OUT_IF" ]]; then
  sudo iptables -t nat -C POSTROUTING -s "$UE_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE 2>/dev/null || \
    sudo iptables -t nat -A POSTROUTING -s "$UE_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE
  sudo iptables -t nat -C POSTROUTING -s "$N6_CENTER_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE 2>/dev/null || \
    sudo iptables -t nat -A POSTROUTING -s "$N6_CENTER_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE
  sudo iptables -t nat -C POSTROUTING -s "$N6_EDGE_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE 2>/dev/null || \
    sudo iptables -t nat -A POSTROUTING -s "$N6_EDGE_SUBNET" -o "$N6_OUT_IF" -j MASQUERADE

  sudo iptables -C FORWARD -s "$UE_SUBNET" -o "$N6_OUT_IF" -j ACCEPT 2>/dev/null || \
    sudo iptables -A FORWARD -s "$UE_SUBNET" -o "$N6_OUT_IF" -j ACCEPT
  sudo iptables -C FORWARD -d "$UE_SUBNET" -i "$N6_OUT_IF" -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || \
    sudo iptables -A FORWARD -d "$UE_SUBNET" -i "$N6_OUT_IF" -m state --state RELATED,ESTABLISHED -j ACCEPT
else
  echo "Warning: N6_OUT_IF is empty; skip host NAT setup" >&2
fi

echo "N6 host setup complete: $N6_CENTER_IF=$N6_CENTER_GW, $N6_EDGE_IF=$N6_EDGE_GW, out=${N6_OUT_IF:-none}"
