#!/bin/sh
docker_embedded_dns_ip='127.0.0.11'

# first we need to detect an IP to use for reaching the docker host
docker_host_ip="$(ip -4 route show default | cut -d' ' -f3)"

# now we can ensure that DNS is configured to use our IP
cp /etc/resolv.conf /etc/resolv.conf.original
sed -e "s/${docker_embedded_dns_ip}/${docker_host_ip}/g" /etc/resolv.conf.original >/etc/resolv.conf
$@