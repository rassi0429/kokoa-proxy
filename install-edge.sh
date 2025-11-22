#!/usr/bin/env bash
set -euo pipefail

# Minimal installer scaffold for edge nodes. It installs dependencies and deploys the agent script.
if [[ $EUID -ne 0 ]]; then
  echo "run as root"
  exit 1
fi

APT_PACKAGES=(nginx wireguard curl jq cron)
echo "Installing packages: ${APT_PACKAGES[*]}"
apt-get update -y
apt-get install -y "${APT_PACKAGES[@]}"

mkdir -p /opt/kokoa-edge
cp scripts/kokoa-edge-agent/poll_config.sh /opt/kokoa-edge/
chmod +x /opt/kokoa-edge/poll_config.sh

cat >/etc/systemd/system/kokoa-edge.service <<'EOF'
[Unit]
Description=Kokoa Edge Agent
After=network-online.target

[Service]
Environment=CONTROL_PLANE_URL=https://control-plane.example.com
Environment=NODE_TOKEN=
Environment=CONFIG_DIR=/etc/nginx/kokoa
ExecStart=/opt/kokoa-edge/poll_config.sh
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now kokoa-edge.service

echo "Edge agent installed. Configure CONTROL_PLANE_URL and NODE_TOKEN in /etc/systemd/system/kokoa-edge.service."
