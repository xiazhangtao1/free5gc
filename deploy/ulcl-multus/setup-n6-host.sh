#!/usr/bin/env bash
set -euo pipefail

N6_MASTER_IF=${N6_MASTER_IF:-wlp128s0}
N6_CENTER_IF=${N6_CENTER_IF:-n6host1}
N6_EDGE_IF=${N6_EDGE_IF:-n6host2}
N6_CENTER_GW=${N6_CENTER_GW:-10.100.100.1/24}
N6_EDGE_GW=${N6_EDGE_GW:-10.100.200.1/24}
N6_CENTER_UPF=${N6_CENTER_UPF:-10.100.100.12}
N6_EDGE_UPF=${N6_EDGE_UPF:-10.100.200.12}
UE_SUBNET=${UE_SUBNET:-10.60.0.0/16}
UE_DEFAULT_UPF=${UE_DEFAULT_UPF:-$N6_EDGE_UPF}
UE_DEFAULT_IF=${UE_DEFAULT_IF:-$N6_EDGE_IF}
UE_EDGE_SUBNET=${UE_EDGE_SUBNET:-}
N6_CENTER_SUBNET=${N6_CENTER_SUBNET:-10.100.100.0/24}
N6_EDGE_SUBNET=${N6_EDGE_SUBNET:-10.100.200.0/24}
N6_OUT_IF=${N6_OUT_IF:-}
N6_POLICY_SNAT=${N6_POLICY_SNAT:-true}
N6_CENTER_DEST_SUBNET=${N6_CENTER_DEST_SUBNET:-192.168.0.0/16}
N6_CENTER_MARK=${N6_CENTER_MARK:-0x1}
N6_EDGE_MARK=${N6_EDGE_MARK:-0x2}
N6_CENTER_TABLE=${N6_CENTER_TABLE:-6101}
N6_EDGE_TABLE=${N6_EDGE_TABLE:-6102}

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
sudo sysctl -w net.ipv4.conf.all.rp_filter=0 >/dev/null
sudo sysctl -w "net.ipv4.conf.$N6_CENTER_IF.rp_filter=0" >/dev/null
sudo sysctl -w "net.ipv4.conf.$N6_EDGE_IF.rp_filter=0" >/dev/null

sudo ip route delete 10.60.0.0/17 via "$N6_CENTER_UPF" dev "$N6_CENTER_IF" 2>/dev/null || true
sudo ip route delete 10.60.128.0/17 via "$N6_EDGE_UPF" dev "$N6_EDGE_IF" 2>/dev/null || true
sudo ip route replace "$UE_SUBNET" via "$UE_DEFAULT_UPF" dev "$UE_DEFAULT_IF"

if [[ -n "$UE_EDGE_SUBNET" ]]; then
  sudo ip route replace "$UE_EDGE_SUBNET" via "$N6_EDGE_UPF" dev "$N6_EDGE_IF"
fi

if ip route show table 7113 >/dev/null 2>&1; then
  ip rule show | grep -q "from $UE_SUBNET lookup 7113" || sudo ip rule add from "$UE_SUBNET" lookup 7113 priority 1100
fi

if [[ "$N6_POLICY_SNAT" == "true" ]]; then
  sudo ip route replace "$UE_SUBNET" via "$N6_CENTER_UPF" dev "$N6_CENTER_IF" table "$N6_CENTER_TABLE"
  sudo ip route replace "$UE_SUBNET" via "$N6_EDGE_UPF" dev "$N6_EDGE_IF" table "$N6_EDGE_TABLE"

  ip rule show | grep -q "fwmark $N6_CENTER_MARK lookup $N6_CENTER_TABLE" || \
    sudo ip rule add fwmark "$N6_CENTER_MARK" table "$N6_CENTER_TABLE" priority 1001
  ip rule show | grep -q "fwmark $N6_EDGE_MARK lookup $N6_EDGE_TABLE" || \
    sudo ip rule add fwmark "$N6_EDGE_MARK" table "$N6_EDGE_TABLE" priority 1002

  sudo iptables -t mangle -N XCN_N6_MARK 2>/dev/null || true
  sudo iptables -t mangle -F XCN_N6_MARK
  sudo iptables -t mangle -C PREROUTING -j XCN_N6_MARK 2>/dev/null || \
    sudo iptables -t mangle -A PREROUTING -j XCN_N6_MARK
  sudo iptables -t mangle -A XCN_N6_MARK -j CONNMARK --restore-mark
  sudo iptables -t mangle -A XCN_N6_MARK -i "$N6_CENTER_IF" -s "$UE_SUBNET" -d "$N6_CENTER_DEST_SUBNET" -j MARK --set-mark "$N6_CENTER_MARK"
  sudo iptables -t mangle -A XCN_N6_MARK -i "$N6_EDGE_IF" -s "$UE_SUBNET" -j MARK --set-mark "$N6_EDGE_MARK"
  sudo iptables -t mangle -A XCN_N6_MARK -m mark ! --mark 0 -j CONNMARK --save-mark

  sudo iptables -t nat -N XCN_N6_SNAT 2>/dev/null || true
  sudo iptables -t nat -F XCN_N6_SNAT
  sudo iptables -t nat -C POSTROUTING -j XCN_N6_SNAT 2>/dev/null || \
    sudo iptables -t nat -A POSTROUTING -j XCN_N6_SNAT
  sudo iptables -t nat -A XCN_N6_SNAT -d "$N6_CENTER_SUBNET" -j RETURN
  sudo iptables -t nat -A XCN_N6_SNAT -d "$N6_EDGE_SUBNET" -j RETURN
  sudo iptables -t nat -A XCN_N6_SNAT -s "$UE_SUBNET" -m mark --mark "$N6_CENTER_MARK" -j MASQUERADE
  sudo iptables -t nat -A XCN_N6_SNAT -s "$UE_SUBNET" -m mark --mark "$N6_EDGE_MARK" -j MASQUERADE

  sudo iptables -N XCN_N6_FORWARD 2>/dev/null || true
  sudo iptables -F XCN_N6_FORWARD
  sudo iptables -C FORWARD -j XCN_N6_FORWARD 2>/dev/null || \
    sudo iptables -A FORWARD -j XCN_N6_FORWARD
  sudo iptables -A XCN_N6_FORWARD -i "$N6_CENTER_IF" -s "$UE_SUBNET" -j ACCEPT
  sudo iptables -A XCN_N6_FORWARD -i "$N6_EDGE_IF" -s "$UE_SUBNET" -j ACCEPT
  sudo iptables -A XCN_N6_FORWARD -o "$N6_CENTER_IF" -d "$UE_SUBNET" -m state --state RELATED,ESTABLISHED -j ACCEPT
  sudo iptables -A XCN_N6_FORWARD -o "$N6_EDGE_IF" -d "$UE_SUBNET" -m state --state RELATED,ESTABLISHED -j ACCEPT
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

echo "N6 host setup complete: $N6_CENTER_IF=$N6_CENTER_GW, $N6_EDGE_IF=$N6_EDGE_GW, defaultUpf=$UE_DEFAULT_UPF, policySnat=$N6_POLICY_SNAT, out=${N6_OUT_IF:-none}"
