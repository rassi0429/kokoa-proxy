#!/usr/bin/env bash
set -euo pipefail

# Skeleton for provisioning an origin WireGuard endpoint.
if [[ $EUID -ne 0 ]]; then
  echo "run as root"
  exit 1
fi

apt-get update -y
apt-get install -y wireguard qrencode

WG_IFACE="${WG_IFACE:-wg0}"
WG_PORT="${WG_PORT:-51820}"
WG_ADDR="${WG_ADDR:-10.20.0.1/24}"

umask 077
wg genkey | tee /etc/wireguard/privatekey | wg pubkey > /etc/wireguard/publickey

cat >/etc/wireguard/${WG_IFACE}.conf <<EOF
[Interface]
Address = ${WG_ADDR}
ListenPort = ${WG_PORT}
PrivateKey = $(cat /etc/wireguard/privatekey)
SaveConfig = true

# Peers will be appended by the control plane.
EOF

systemctl enable --now wg-quick@${WG_IFACE}
echo "WireGuard origin ready on ${WG_IFACE}. Add peers using wg set or via the control plane API."
