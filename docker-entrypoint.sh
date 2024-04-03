#!/bin/sh

set -eu

# DNS fixup adapted from kind
# https://github.com/kubernetes-sigs/kind/blob/7568bf728147c1253e651f25edfd0e0a75534b8a/images/base/files/usr/local/bin/entrypoint#L447-L487

# well-known docker embedded DNS is at 127.0.0.11:53
docker_embedded_dns_ip=127.0.0.11

# first we need to detect an IP to use for reaching the docker host
docker_host_ip=$(timeout 5 getent ahostsv4 host.docker.internal | head -n1 | cut -d' ' -f1 || true)
# if the ip doesn't exist or is a loopback address use the default gateway
case "$docker_host_ip" in
'' | 127.*) docker_host_ip=$(ip -4 route show default | cut -d' ' -f3) ;;
esac

for iptables in iptables iptables-nft; do
  # patch docker's iptables rules to switch out the DNS IP
  "$iptables"-save \
    | sed \
      `# switch docker DNS DNAT rules to our chosen IP` \
      -e "s/-d ${docker_embedded_dns_ip}/-d ${docker_host_ip}/g" \
      `# we need to also apply these rules to non-local traffic (from pods)` \
      -e 's/-A OUTPUT \(.*\) -j DOCKER_OUTPUT/\0\n-A PREROUTING \1 -j DOCKER_OUTPUT/' \
      `# switch docker DNS SNAT rules rules to our chosen IP` \
      -e "s/--to-source :53/--to-source ${docker_host_ip}:53/g" \
      `# nftables incompatibility between 1.8.8 and 1.8.7 omit the --dport flag on DNAT rules` \
      `# ensure --dport on DNS rules, due to https://github.com/kubernetes-sigs/kind/issues/3054` \
      -e "s/p -j DNAT --to-destination ${docker_embedded_dns_ip}/p --dport 53 -j DNAT --to-destination ${docker_embedded_dns_ip}/g" \
    | "$iptables"-restore
done

# now we can ensure that DNS is configured to use our IP
cp /etc/resolv.conf /etc/resolv.conf.original
sed -e "s/${docker_embedded_dns_ip}/${docker_host_ip}/g" /etc/resolv.conf.original >/etc/resolv.conf

# write config from environment variable
if [ -n "${K0S_CONFIG-}" ]; then
  mkdir -p /etc/k0s
  printf %s "$K0S_CONFIG" >/etc/k0s/config.yaml
fi

exec "$@"
