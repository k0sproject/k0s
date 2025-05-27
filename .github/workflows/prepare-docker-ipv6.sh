#!/usr/bin/env sh

set -eu

echo '{"ipv6": true,"fixed-cidr-v6": "2001:db8:1::/64"}' | sudo tee /etc/docker/daemon.json
sudo systemctl restart docker
docker network inspect bridge-ipv6 >/dev/null 2>&1 || \
docker network create \
  --driver bridge \
  --ipv6 --subnet="2001:db8:2::/64" \
  --gateway 2001:db8:2::1 \
  -o com.docker.network.enable_ipv4=false \
  bridge-ipv6
