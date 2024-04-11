//go:build unix

/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/k0sproject/k0s/internal/pkg/users"
	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// Keepalived is the controller for the keepalived process in the control plane load balancing
type Keepalived struct {
	K0sVars         *config.CfgVars
	Config          *k0sAPI.ControlPlaneLoadBalancingSpec
	DetailedLogging bool
	uid             int
	supervisor      *supervisor.Supervisor
	log             *logrus.Entry
	configFilePath  string
}

// Init extracts the needed binaries and creates the directories
func (k *Keepalived) Init(_ context.Context) error {
	if k.Config == nil {
		return nil
	}
	k.log = logrus.WithField("component", "CPLB")

	var err error
	k.uid, err = users.GetUID(constant.KeepalivedUser)
	if err != nil {
		k.log.Warnf("Unable to get %s UID running keepalived as root: %v", constant.KeepalivedUser, err)
	}

	k.configFilePath = filepath.Join(k.K0sVars.RunDir, "keepalived.conf")
	return assets.Stage(k.K0sVars.BinDir, "keepalived", constant.BinDirMode)
}

// Start generates the keepalived configuration and starts the keepalived process
func (k *Keepalived) Start(_ context.Context) error {
	if k.Config == nil {
		return nil
	}

	if err := k.configureDummy(); err != nil {
		return fmt.Errorf("failed to configure dummy interface: %w", err)
	}

	if err := k.Config.ValidateVRRPInstances(nil); err != nil {
		return fmt.Errorf("failed to validate VRRP instances: %w", err)
	}
	if err := k.generateKeepalivedTemplate(); err != nil {
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

	k.log.Infoln("Starting keepalived")
	k.supervisor = &supervisor.Supervisor{
		Name:    "keepalived",
		BinPath: assets.BinPath("keepalived", k.K0sVars.BinDir),
		Args:    args,
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		UID:     k.uid,
	}
	return k.supervisor.Supervise()
}

// Stops keepalived and cleans up the virtual IPs. This is done so that if the
// k0s controller is stopped, it can still reach the other APIservers on the VIP
func (k *Keepalived) Stop() error {
	k.log.Infof("Stopping keepalived")
	if err := k.supervisor.Stop(); err != nil {
		// Failed to stop keepalived. Don't delete the VIP, just in case.
		return fmt.Errorf("failed to stop keepalived: %w", err)
	}

	k.log.Infof("Deleting dummy interface")
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

// configureDummy creates the dummy interface and sets the virtual IPs on it.
func (k *Keepalived) configureDummy() error {
	vips := []string{}
	for _, vi := range k.Config.VRRPInstances {
		for _, vip := range vi.VirtualIPs {
			vips = append(vips, vip)
		}
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
	for i := 0; i < len(linkAddrs); i++ {
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

func (k *Keepalived) generateKeepalivedTemplate() error {
	f, err := os.OpenFile(k.configFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(0500))
	if err != nil {
		return fmt.Errorf("failed to open keepalived config file: %w", err)
	}
	defer f.Close()

	template, err := template.New("keepalived").Parse(keepalivedConfigTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse keepalived template: %w", err)
	}

	kc := keepalivedConfig{
		VRRPInstances: k.Config.VRRPInstances,
	}
	if err = template.Execute(f, kc); err != nil {
		return fmt.Errorf("failed to execute keepalived template: %w", err)
	}

	// TODO: Do we really need to this every single time?
	if err = os.Chown(k.configFilePath, k.uid, -1); err != nil {
		return fmt.Errorf("failed to chown keepalived config file: %w", err)
	}
	if err = os.Chmod(k.configFilePath, fs.FileMode(0400)); err != nil {
		return fmt.Errorf("failed to chmod keepalived config file: %w", err)
	}
	return nil
}

// keepalivedConfig contains all the information required by the
// KeepalivedConfigTemplate.
// Right now this struct doesn't make sense right now but we need this for the
// future virtual_server support.
type keepalivedConfig struct {
	VRRPInstances []k0sAPI.VRRPInstance
}

const dummyLinkName = "dummyvip0"
const keepalivedConfigTemplate = `
{{ range .VRRPInstances }}
vrrp_instance {{ .Name }} {
	# All servers must have state BACKUP so that when a new server comes up
	# it doesn't perform a failover. This must be combined with the priority.
    state BACKUP
    # Make sure the interface is aligned with your server's network interface
    interface {{ .Interface }}
    # The virtual router ID must be unique to each VRRP instance that you define
    virtual_router_id {{ .VirtualRouterID }}
    # All servers have the same priority so that when a new one comes up we don't
    # do a failover
    priority 200
#   advertisement interval, 1 second by default
    advert_int {{ .AdvertInterval }}
    authentication {
        auth_type PASS
        auth_pass {{ .AuthPass }}
    }
    virtual_ipaddress {
	    {{ range .VirtualIPs }}
		{{ . }}
		{{ end }}
    }
}
{{ end }}
`
