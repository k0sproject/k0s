#!/bin/sh

# Ensure we have some semi-random machine-id
if [ ! -f  /etc/machine-id ]; then
    dd if=/dev/urandom status=none bs=16 count=1 | md5sum | cut -d' ' -f1 > /etc/machine-id
fi

# Network fixups adapted from kind: https://github.com/kubernetes-sigs/kind/blob/master/images/base/files/usr/local/bin/entrypoint#L176
docker_embedded_dns_ip='127.0.0.11'
# first we need to detect an IP to use for reaching the docker host
docker_host_ip="$(ip -4 route show default | cut -d' ' -f3)"

# patch docker's iptables rules to switch out the DNS IP
iptables-save \
  | sed \
    `# switch docker DNS DNAT rules to our chosen IP` \
    -e "s/-d ${docker_embedded_dns_ip}/-d ${docker_host_ip}/g" \
    `# we need to also apply these rules to non-local traffic (from pods)` \
    -e 's/-A OUTPUT \(.*\) -j DOCKER_OUTPUT/\0\n-A PREROUTING \1 -j DOCKER_OUTPUT/' \
    `# switch docker DNS SNAT rules rules to our chosen IP` \
    -e "s/--to-source :53/--to-source ${docker_host_ip}:53/g"\
  | iptables-restore

# now we can ensure that DNS is configured to use our IP
cp /etc/resolv.conf /etc/resolv.conf.original
sed -e "s/${docker_embedded_dns_ip}/${docker_host_ip}/g" /etc/resolv.conf.original >/etc/resolv.conf

exec $@