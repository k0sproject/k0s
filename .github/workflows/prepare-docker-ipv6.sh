#!/usr/bin/env sh

set -eu

echo '{"ipv6": true,"fixed-cidr-v6": "2001:db8:1::/64"}' | sudo tee /etc/docker/daemon.json
sudo systemctl restart docker
