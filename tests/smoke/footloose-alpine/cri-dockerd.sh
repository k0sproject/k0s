#!/sbin/openrc-run
supervisor=supervise-daemon
name="cri-dockerd"
description="cri-dockerd"
command=/usr/local/bin/cri-dockerd
command_args="--network-plugin=cni --cni-conf-dir=/etc/cni/net.d --cni-bin-dir=/opt/cni/bin"
name=$(basename $(readlink -f $command))
supervise_daemon_args="--stdout /var/log/${name}.log --stderr /var/log/${name}.err"

: "${rc_ulimit=-n 1048576 -u unlimited}"
depend() { 
        need net 
        use dns 
        after firewall
}