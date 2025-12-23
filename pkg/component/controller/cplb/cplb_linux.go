// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/users"
	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/controller/cplb/tcpproxy"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	iptablesCommandAppend = "-A"
	iptablesCommandDelete = "-D"
)

// Keepalived is the controller for the keepalived process in the control plane load balancing
type Keepalived struct {
	K0sVars         *config.CfgVars
	Config          *k0sAPI.KeepalivedSpec
	DetailedLogging bool
	LogConfig       bool
	APIPort         int
	KubeConfigPath  string

	keepalivedConfig       *keepalivedConfig
	supervisor             *supervisor.Supervisor
	uid                    int
	executablePath         string
	log                    *logrus.Entry
	configFilePath         string
	virtualServersFilePath string
	reconciler             *CPLBReconciler
	updateCh               chan struct{}
	reconcilerDone         chan struct{}
	proxy                  tcpproxy.Proxy
}

// Init extracts the needed binaries and creates the directories
func (k *Keepalived) Init(_ context.Context) error {
	if k.Config == nil {
		return nil
	}
	k.log = logrus.WithField("component", "CPLB")

	var err error
	k.uid, err = users.LookupUID(constant.KeepalivedUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.KeepalivedUser, err)
		k.uid = users.RootUID
		k.log.WithError(err).Warn("Running keepalived as root")
	}

	k.configFilePath = filepath.Join(k.K0sVars.RunDir, "keepalived.conf")
	k.virtualServersFilePath = filepath.Join(k.K0sVars.RunDir, "keepalived-virtualservers-generated.conf")
	k.executablePath, err = assets.StageExecutable(k.K0sVars.BinDir, "keepalived")
	return err
}

// Start generates the keepalived configuration and starts the keepalived process
func (k *Keepalived) Start(ctx context.Context) error {
	if k.Config == nil || (len(k.Config.VRRPInstances) == 0 && len(k.Config.VirtualServers) == 0) {
		k.log.Warn("No VRRP instances or virtual servers defined, skipping keepalived start")
		return nil
	}

	// IPv6 clusters need labels. We only do this process for IPv6 VIPs
	if err := k.setVirtualIPAddressLabels(); err != nil {
		return fmt.Errorf("failed to set address labels: %w", err)
	}

	// We only need the dummy interface when using IPVS.
	if len(k.Config.VirtualServers) > 0 {
		if err := k.configureDummy(); err != nil {
			return fmt.Errorf("failed to configure dummy interface: %w", err)
		}
	}

	if !k.Config.DisableLoadBalancer && (len(k.Config.VRRPInstances) > 0 || len(k.Config.VirtualServers) > 0) {
		k.log.Info("Starting CPLB reconciler")
		updateCh := make(chan struct{}, 1)
		k.reconciler = NewCPLBReconciler(k.KubeConfigPath, k.APIPort, updateCh)
		k.updateCh = updateCh
		if err := k.reconciler.Start(); err != nil {
			return fmt.Errorf("failed to start CPLB reconciler: %w", err)
		}
	}

	k.keepalivedConfig = &keepalivedConfig{
		VRRPInstances:  k.Config.VRRPInstances,
		VirtualServers: k.Config.VirtualServers,
		APIServerPort:  k.APIPort,
		K0sBin:         escapeSingleQuotes(os.Args[0]),
		RunDir:         escapeSingleQuotes(k.K0sVars.RunDir),
	}

	if len(k.Config.VirtualServers) > 0 {
		k.keepalivedConfig.IPVSLoadBalancer = true

	}

	// In order to make the code simpler, we always create the keepalived template
	// without the virtual servers, before starting the reconcile loop

	templ := template.Must(template.New("keepalived").Parse(keepalivedVRRPConfigTemplate))
	if err := k.generateTemplate(templ, k.configFilePath); err != nil {
		return fmt.Errorf("failed to generate keepalived template: %w", err)
	}

	args := []string{
		"--dont-fork",
		"--use-file",
		k.configFilePath,
		"--no-syslog",
		"--log-console",
	}

	if k.DetailedLogging {
		args = append(args, "--log-detail")
	}
	if k.LogConfig {
		args = append(args, "--dump-conf")
	}

	k.log.Infoln("Starting keepalived")
	k.supervisor = &supervisor.Supervisor{
		Name:    "keepalived",
		BinPath: k.executablePath,
		Args:    args,
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		UID:     k.uid,
	}

	if k.reconciler != nil {
		reconcilerDone := make(chan struct{})
		k.reconcilerDone = reconcilerDone
		go func() {
			defer close(reconcilerDone)
			if len(k.Config.VirtualServers) > 0 {
				k.watchReconcilerUpdatesKeepalived()
			} else {
				if err := k.watchReconcilerUpdatesReverseProxy(ctx); err != nil {
					k.log.WithError(err).Error("failed to watch reconciler updates")
				}
			}
		}()
	}

	return k.supervisor.Supervise(ctx)
}

// Stops keepalived and cleans up the virtual IPs. This is done so that if the
// k0s controller is stopped, it can still reach the other APIservers on the VIP
func (k *Keepalived) Stop() error {
	if k.reconciler != nil {
		k.log.Info("Stopping cplb-reconciler")
		k.reconciler.Stop()
		close(k.updateCh)
		<-k.reconcilerDone
	}

	k.log.Info("Stopping keepalived")
	if err := k.supervisor.Stop(); err != nil {
		k.log.WithError(err).Error("Failed to stop executable")
	}

	if len(k.Config.VirtualServers) > 0 {
		k.log.Info("Deleting dummy interface")
		link, err := netlink.LinkByName(dummyLinkName)
		if err != nil {
			if errors.As(err, &netlink.LinkNotFoundError{}) {
				return nil
			}
			k.log.Errorf("failed to get link by name %s. Attempting to delete it anyway: %v", dummyLinkName, err)
			link = &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: dummyLinkName,
				},
			}
		}
		return netlink.LinkDel(link)
	}
	if err := k.proxy.Close(); err != nil {
		return fmt.Errorf("failed to close proxy: %w", err)
	}

	// Only clean iptables rules if we are using the userspace reverse proxy
	return k.redirectToProxyIPTables(iptablesCommandDelete)
}

// configureDummy creates the dummy interface and sets the virtual IPs on it.
func (k *Keepalived) configureDummy() error {
	var vips []string
	for _, vi := range k.Config.VRRPInstances {
		vips = append(vips, vi.VirtualIPs...)
	}

	if len(vips) > 0 {
		k.log.Infof("Creating dummy interface")
		if err := k.ensureDummyInterface(dummyLinkName); err != nil {
			k.log.Errorf("failed to create dummy interface: %v", err)
		}
		// If the dummy interface fails, attempt to define the addresses just
		// in case.
		if err := k.ensureLinkAddresses(dummyLinkName, vips); err != nil {
			return fmt.Errorf("failed to ensure link addresses: %w", err)
		}
	}
	return nil
}

func (k *Keepalived) ensureDummyInterface(linkName string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		// There are multiple reasons why the link may not be returned besides
		// it not existing. If we don't know what failed log it and attempt to
		// create the link anyway.
		if !errors.As(err, &netlink.LinkNotFoundError{}) {
			k.log.Warnf("failed to get link by name %s. Attempting to create it anyway: %v", linkName, err)
		}
		return k.createDummyInterface(linkName)
	}

	if _, isDummy := link.(*netlink.Dummy); isDummy {
		return nil
	}

	// This happens if the interface exists but it's not a dummy interface
	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete %s: %w", linkName, err)
	}

	return k.createDummyInterface(linkName)
}

func (k *Keepalived) createDummyInterface(linkName string) error {
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: linkName,
		},
	}
	return netlink.LinkAdd(link)
}

func (k *Keepalived) ensureLinkAddresses(linkName string, expectedAddresses []string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link by name %s: %w", linkName, err)
	}

	linkAddrs, strAddrs, err := k.getLinkAddresses(link)
	if err != nil {
		return fmt.Errorf("failed to get addresses for link %s: %w", linkName, err)
	}

	// Remove unexpected addresses
	for i := range linkAddrs {
		strAddr := strAddrs[i]
		linkAddr := linkAddrs[i]
		if !slices.Contains(expectedAddresses, strAddrs[i]) {
			k.log.Infof("Deleting address %s from link %s", strAddr, linkName)
			if err = netlink.AddrDel(link, &linkAddr); err != nil {
				return fmt.Errorf("failed to delete address %s from link %s: %w", linkAddr.IPNet.String(), linkName, err)
			}
		}
	}

	// Add missing expected addresses
	for _, addr := range expectedAddresses {
		if !slices.Contains(strAddrs, addr) {
			if err = k.setLinkIP(addr, linkName, link); err != nil {
				return fmt.Errorf("failed to add address %s to link %s: %w", addr, linkName, err)
			}
		}
	}

	return nil
}

func (k *Keepalived) setLinkIP(addr string, linkName string, link netlink.Link) error {
	ipAddr, _, err := net.ParseCIDR(addr)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR %s: %w", addr, err)
	}

	var mask net.IPMask
	if ipAddr.To4() != nil {
		mask = net.CIDRMask(32, 32)
	} else {
		mask = net.CIDRMask(128, 128)
	}

	linkAddr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ipAddr,
			Mask: mask,
		},
	}

	k.log.Infof("Adding address %s to link %s", addr, linkName)
	if err := netlink.AddrAdd(link, linkAddr); err != nil {
		return fmt.Errorf("failed to add address %s to link %s: %w", addr, linkName, err)
	}
	return nil
}

func (*Keepalived) getLinkAddresses(link netlink.Link) ([]netlink.Addr, []string, error) {
	linkAddrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list addresses for link %s: %w", link.Attrs().Name, err)
	}

	strAddrs := make([]string, len(linkAddrs))
	for i, addr := range linkAddrs {
		strAddrs[i] = addr.IPNet.String()
	}
	return linkAddrs, strAddrs, nil
}

// setVirtualIPAddressLabels sets the labels for the vips to vrrp.AddressLabel
// so that the real IP is preferred.
func (k *Keepalived) setVirtualIPAddressLabels() error {
	for _, vrrp := range k.Config.VRRPInstances {
		for _, vip := range vrrp.VirtualIPs {
			// Only set labels for IPv6 addresses
			ipAddr, _, err := net.ParseCIDR(vip)
			if err != nil {
				return fmt.Errorf("failed to parse CIDR %s: %w", vip, err)
			}

			// Only set labels for IPv6 addresses
			if ipAddr.To4() != nil {
				continue
			}

			// Set address label for IPv6 VIP
			if err := setAddressLabel(ipAddr, vrrp.AddressLabel); err != nil {
				return fmt.Errorf("failed to set address label for %s: %w", ipAddr, err)
			}
		}
	}
	return nil
}

func (k *Keepalived) generateTemplate(templ *template.Template, path string) error {
	if err := file.AtomicWithTarget(path).
		WithPermissions(0400).
		WithOwner(k.uid).
		Do(func(unbuffered file.AtomicWriter) error {
			w := bufio.NewWriter(unbuffered)
			if err := templ.Execute(w, k.keepalivedConfig); err != nil {
				return err
			}
			return w.Flush()
		}); err != nil {
		return fmt.Errorf("failed to write keepalived config file: %w", err)
	}

	return nil
}

func (k *Keepalived) watchReconcilerUpdatesReverseProxy(ctx context.Context) error {
	k.proxy = tcpproxy.Proxy{}
	// We don't know how long until we get the first update, so initially we
	// forward everything to localhost
	k.proxy.SetRoutes(fmt.Sprintf(":%d", k.Config.UserSpaceProxyPort), []tcpproxy.Route{tcpproxy.To(fmt.Sprintf("127.0.0.1:%d", k.APIPort))})

	if err := k.proxy.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	k.log.Info("Waiting for the first cplb-reconciler update")

	select {
	case <-ctx.Done():
		return errors.New("context canceled while starting the reverse proxy")
	case <-k.updateCh:
	}
	k.setProxyRoutes()

	// Do not create the iptables rules until we have the first update and the
	// proxy is running
	if err := k.redirectToProxyIPTables(iptablesCommandAppend); err != nil {
		k.log.Fatal(err)
	}

	for range k.updateCh {
		k.setProxyRoutes()
	}
	return nil
}

func (k *Keepalived) setProxyRoutes() {
	routes := []tcpproxy.Route{}
	port := strconv.Itoa(k.APIPort)
	for _, addr := range k.reconciler.GetIPs() {
		routes = append(routes, tcpproxy.To(net.JoinHostPort(addr, port)))
	}

	if len(routes) == 0 {
		k.log.Error("No API servers available, leave previous configuration")
		return
	}
	k.proxy.SetRoutes(fmt.Sprintf(":%d", k.Config.UserSpaceProxyPort), routes)
}

func (k *Keepalived) redirectToProxyIPTables(op string) error {
	for _, vrrp := range k.Config.VRRPInstances {
		for _, vipCIDR := range vrrp.VirtualIPs {
			vip := strings.Split(vipCIDR, "/")[0]

			cmdArgs := []string{
				"-t", "nat", op, "PREROUTING", "-p", "tcp",
				"-d", vip, "--dport", strconv.Itoa(k.APIPort),
				"-j", "REDIRECT", "--to-port", strconv.Itoa(k.Config.UserSpaceProxyPort),
			}

			switch op {
			case iptablesCommandAppend:
				k.log.Infof("Adding iptables rule to redirect %s", vip)
			case iptablesCommandDelete:
				k.log.Infof("Deleting iptables rule to redirect %s", vip)
			}

			iptablesBin := "iptables"
			if ip := net.ParseIP(vip); ip != nil && ip.To4() == nil {
				iptablesBin = "ip6tables"
			}
			cmd := exec.Command(filepath.Join(k.K0sVars.BinDir, iptablesBin), cmdArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to execute iptables command: %w, output: %s", err, output)
			}
		}
	}
	return nil
}

func (k *Keepalived) watchReconcilerUpdatesKeepalived() {
	// Wait for the supervisor to start keepalived before
	// watching for endpoint changes
	process := k.supervisor.GetProcess()
	for i := 0; process == nil; i++ {
		if i > 3 {
			k.log.Error("failed to start keepalived, supervisor process is nil")
			return
		}
		k.log.Info("Waiting for keepalived to start")
		time.Sleep(5 * time.Second)
		process = k.supervisor.GetProcess()
	}

	k.log.Info("started watching cplb-reconciler updates")
	templ := template.Must(template.New("keepalived").Parse(keepalivedVirtualServersConfigTemplate))
	for range k.updateCh {
		k.keepalivedConfig.RealServers = k.reconciler.GetIPs()
		k.log.Infof("cplb-reconciler update, got %s", k.keepalivedConfig.RealServers)
		if err := k.generateTemplate(templ, k.virtualServersFilePath); err != nil {
			k.log.Errorf("failed to generate keepalived template: %v", err)
			continue
		}

		process := k.supervisor.GetProcess()
		if err := process.Signal(syscall.SIGHUP); err != nil {
			k.log.Errorf("failed to send SIGHUP to keepalived: %v", err)
		}
	}
	k.log.Info("stopped watching cplb-reconciler updates")
}

// escapeSingleQuotes escapes single quotes in a string for use in the keepalived
// template.
func escapeSingleQuotes(s string) string {
	str := strings.ReplaceAll(s, `\'`, `'`)
	return strings.ReplaceAll(str, `'`, `\'`)
}

// keepalivedConfig contains all the information required by the
// KeepalivedConfigTemplate.
// Right now this struct doesn't make sense right now but we need this for the
// future virtual_server support.
type keepalivedConfig struct {
	VRRPInstances    []k0sAPI.VRRPInstance
	VirtualServers   []k0sAPI.VirtualServer
	RealServers      []string
	APIServerPort    int
	IPVSLoadBalancer bool
	K0sBin           string
	RunDir           string
}

const dummyLinkName = "dummyvip0"

const keepalivedVRRPConfigTemplate = `
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
    notify_master "'{{ $k0s }}' keepalived-setstate -r '{{ $runDir }}' -s MASTER"
    notify_backup "'{{ $k0s }}' keepalived-setstate -r '{{ $runDir }}' -s BACKUP"
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

const keepalivedVirtualServersConfigTemplate = `
{{ $APIServerPort := .APIServerPort }}
{{ $RealServers := .RealServers }}
{{ if gt (len $RealServers) 0 }}
{{ range .VirtualServers }}
virtual_server {{ .IPAddress }} {{ $APIServerPort }} {
    delay_loop {{ .DelayLoop.Seconds }}
    lb_algo {{ .LBAlgo }}
    lb_kind {{ .LBKind }}
    persistence_timeout {{ .PersistenceTimeoutSeconds }}
    protocol TCP

    {{ range $RealServers }}
    real_server {{ . }} {{ $APIServerPort }} {
        weight 1
        TCP_CHECK {
            warmup 0
            retry 1
            connect_timeout 3
            connect_port {{ $APIServerPort }}
        }
    }
    {{end}}
}
{{ end }}
{{ end }}
`
