// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

const KeepalivedVRRPConfigTemplate = `
{{ $ipvsLoadBalancer := .IPVSLoadBalancer }}
{{ $k0s := .K0sBin }}
{{ $runDir := .RunDir }}
{{ $VRRPInstancesLen := len .VRRPInstances }}
{{ range $i, $instance := .VRRPInstances }}
vrrp_instance k0s-vip-{{$i}} {
    # All servers must have state BACKUP so that when a new server comes up
    # it doesn't perform a failover. This must be combined with the priority.
    state BACKUP
    # Make sure the interface is aligned with your server's network interface.
    interface {{ .Interface }}
    # The virtual router ID must be unique to each VRRP instance that you define.
    virtual_router_id {{ $instance.VirtualRouterID }}
    # All servers have the same priority so that when a new one comes up we don't
    # do a failover.
    priority 200

    {{ if and ($ipvsLoadBalancer) (eq $VRRPInstancesLen 1) }}
    # Required to prevent routing loops when we use keepalived
    # virtual_servers: https://github.com/k0sproject/k0s/issues/5178
    # The scripts may finish in any order, so the virtual IPs on the interface decide the state.
    notify_master "'{{ $k0s }}' keepalived-setstate -r '{{ $runDir }}' -s MASTER --interface '{{ $instance.Interface }}'{{ range $instance.VirtualIPs }} --virtual-ip '{{ . }}'{{ end }}"
    notify_backup "'{{ $k0s }}' keepalived-setstate -r '{{ $runDir }}' -s BACKUP --interface '{{ $instance.Interface }}'{{ range $instance.VirtualIPs }} --virtual-ip '{{ . }}'{{ end }}"
    {{ end }}

	#advertisement interval, 1 second by default
    advert_int {{ $instance.AdvertIntervalSeconds }}
    authentication {
        auth_type PASS
        auth_pass {{ $instance.AuthPass }}
    }
    virtual_ipaddress {
        {{ range $instance.VirtualIPs }}
        {{ . }}
        {{ end }}
    }
    {{ if .UnicastPeers }}
    unicast_src_ip {{ .UnicastSourceIP }}
    unicast_peer {
        {{ range .UnicastPeers }}
        {{ . }}
        {{ end }}
    }
    {{ else}}
    {{ end }}
}
{{ end }}
{{ if $ipvsLoadBalancer }}
{{ if eq $VRRPInstancesLen 1 }}
# This include is commented by default and is only used after becoming master
# so that we prevent routing looops: https://github.com/k0sproject/k0s/issues/5178
include keepalived-virtualservers-consumed.conf
{{ else}}
# If there is more than one VRRP instance, we need to always have the servers list
# because we cannot guarantee that the masters will always be in the same host.
include keepalived-virtualservers-generated.conf
{{ end }}
{{ end }}
`

const KeepalivedVirtualServersConfigTemplate = `
{{ $LoadBalancedPorts := .LoadBalancedPorts }}
{{ $RealServers := .RealServers }}
{{ if gt (len $RealServers) 0 }}
{{ range $VirtualServer := .VirtualServers }}
{{ range $Port := $LoadBalancedPorts }}
virtual_server {{ $VirtualServer.IPAddress }} {{ $Port }} {
    delay_loop {{ $VirtualServer.DelayLoop.Seconds }}
    lb_algo {{ $VirtualServer.LBAlgo }}
    lb_kind {{ $VirtualServer.LBKind }}
    persistence_timeout {{ $VirtualServer.PersistenceTimeoutSeconds }}
    protocol TCP

    {{ range $RealServers }}
    real_server {{ . }} {{ $Port }} {
        weight 1
        TCP_CHECK {
            warmup 0
            retry 1
            connect_timeout 3
            connect_port {{ $Port }}
        }
    }
    {{end}}
}
{{ end }}
{{ end }}
{{ end }}
`
