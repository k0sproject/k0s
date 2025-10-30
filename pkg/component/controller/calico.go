// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/k0sproject/k0s/static"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/sync/value"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*Calico)(nil)
var _ manager.Reconciler = (*Calico)(nil)

var calicoCRDOnce sync.Once

// Calico is the Component interface implementation to manage Calico
type Calico struct {
	log                  logrus.FieldLogger
	nodeConfig           calicoNodeConfig
	primaryAddressFamily v1beta1.PrimaryAddressFamilyType
	manifestsDir         string
	hasWindowsNodes      func() (*bool, <-chan struct{})

	config value.Latest[*calicoClusterConfig]
	stop   func()
}

type calicoMode string

const (
	calicoModeBIRD  calicoMode = "bird"
	calicoModeVXLAN calicoMode = "vxlan"
)

type calicoConfig struct {
	*calicoNodeConfig
	*calicoClusterConfig
	IncludeWindows bool
}

type calicoNodeConfig struct {
	APIServer       *k0snet.HostPort
	ServiceCIDRIPv4 string
	ClusterDNSIP    string
}

type calicoClusterConfig struct {
	MTU                  int
	Mode                 calicoMode
	VxlanPort            int
	VxlanVNI             int
	ClusterCIDRIPv4      string
	ClusterCIDRIPv6      string
	EnableWireguard      bool
	FlexVolumeDriverPath string
	EnableIPv4           bool
	EnableIPv6           bool
	EnvVars              map[string]string

	CalicoCNIImage             string
	CalicoCNIWindowsImage      string
	CalicoNodeImage            string
	CalicoNodeWindowsImage     string
	CalicoKubeControllersImage string
	Overlay                    string
	IPAutodetectionMethod      string
	IPV6AutodetectionMethod    string
	PullPolicy                 string
}

// NewCalico creates new Calico reconciler component
func NewCalico(nodeConfig *v1beta1.ClusterConfig, manifestsDir string, hasWindowsNodes func() (*bool, <-chan struct{})) (*Calico, error) {
	dnsAddress, err := nodeConfig.Spec.Network.DNSAddress()
	if err != nil {
		return nil, err
	}

	apiServer, err := nodeConfig.Spec.API.APIServerHostPort()
	if err != nil {
		return nil, err
	}

	return &Calico{
		log: logrus.WithFields(logrus.Fields{"component": "calico"}),
		nodeConfig: calicoNodeConfig{
			APIServer:       apiServer,
			ServiceCIDRIPv4: nodeConfig.Spec.Network.ServiceCIDR,
			ClusterDNSIP:    dnsAddress,
		},
		primaryAddressFamily: nodeConfig.PrimaryAddressFamily(),
		manifestsDir:         manifestsDir,
		hasWindowsNodes:      hasWindowsNodes,
	}, nil
}

// Init implements [manager.Component].
func (c *Calico) Init(context.Context) error {
	return errors.Join(
		dir.Init(filepath.Join(c.manifestsDir, "calico_init"), constant.ManifestsDirMode),
		dir.Init(filepath.Join(c.manifestsDir, "calico"), constant.ManifestsDirMode),
	)
}

// Start implements [manager.Component].
func (c *Calico) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		config, configChanged := c.config.Peek()
		hasWin, hasWinChanged := c.hasWindowsNodes()

		var retry <-chan time.Time

		for {
			switch {
			case config == nil, hasWin == nil:
				c.log.Debug("Waiting for configuration")

			default:
				retry = nil
				if err := c.processConfigChanges(&calicoConfig{
					calicoNodeConfig:    &c.nodeConfig,
					calicoClusterConfig: config,
					IncludeWindows:      *hasWin,
				}); err != nil {
					retry = time.After(10 * time.Second)
					c.log.WithError(err).Error("Failed to process configuration changes, retrying in 10 seconds")
				} else {
					c.log.Info("Processed configuration changes")
				}
			}

		waitForUpdates:
			for {
				select {
				case <-configChanged:
					var newConfig *calicoClusterConfig
					newConfig, configChanged = c.config.Peek()
					if updateIfChanged(&config, newConfig) {
						c.log.Info("Cluster configuration changed")
						break waitForUpdates
					} else {
						c.log.Debug("Cluster config unchanged")
					}

				case <-hasWinChanged:
					var newHasWin *bool
					newHasWin, hasWinChanged = c.hasWindowsNodes()
					if updateIfChanged(&hasWin, newHasWin) {
						c.log.Info("Windows nodes changed")
						break waitForUpdates
					} else {
						c.log.Debug("Windows nodes unchanged")
					}

				case <-retry:
					c.log.Debug("Attempting to process configuration again")
					break waitForUpdates

				case <-ctx.Done():
					return
				}
			}
		}
	}()

	c.stop = func() { cancel(); <-done }
	return nil

}

func (c *Calico) dumpCRDs() error {
	var emptyStruct struct{}

	// Write the CRD definitions only at "boot", they do not change during runtime
	crds, err := fs.ReadDir(static.CalicoManifests, "CustomResourceDefinition")
	if err != nil {
		return err
	}

	for _, entry := range crds {
		filename := entry.Name()
		manifestName := "calico-crd-" + filename

		output := bytes.NewBuffer([]byte{})

		contents, err := fs.ReadFile(static.CalicoManifests, path.Join("CustomResourceDefinition", filename))
		if err != nil {
			return fmt.Errorf("failed to fetch crd %s: %w", filename, err)
		}

		tw := templatewriter.TemplateWriter{
			Name:     "calico-crd-" + strings.TrimSuffix(filename, filepath.Ext(filename)),
			Template: string(contents),
			Data:     emptyStruct,
		}
		if err := tw.WriteToBuffer(output); err != nil {
			return fmt.Errorf("failed to write calico crd manifests %s: %w", manifestName, err)
		}

		if err := file.AtomicWithTarget(filepath.Join(c.manifestsDir, "calico_init", manifestName)).
			WithPermissions(constant.CertMode).
			Write(output.Bytes()); err != nil {
			return fmt.Errorf("failed to save calico crd manifest %s: %w", manifestName, err)
		}
	}

	return nil
}

func (c *Calico) processConfigChanges(newConfig *calicoConfig) error {
	manifestDirectories, err := fs.ReadDir(static.CalicoManifests, ".")
	if err != nil {
		return fmt.Errorf("error retrieving calico manifests: %w, will retry", err)
	}

	for _, entry := range manifestDirectories {
		dir := entry.Name()
		// CRDs are handled separately on boot
		if dir == "CustomResourceDefinition" {
			continue
		}
		manifestPaths, err := fs.ReadDir(static.CalicoManifests, dir)
		if err != nil {
			return fmt.Errorf("error retrieving calico manifests: %w, will retry", err)
		}

		tryAndLog := func(name string, e error) {
			if e != nil {
				c.log.Errorf("failed to write manifest %s: %v, will retry", name, e)
			}
		}

		for _, entry := range manifestPaths {
			filename := entry.Name()
			manifestName := fmt.Sprintf("calico-%s-%s", dir, filename)
			output := bytes.NewBuffer([]byte{})
			contents, err := fs.ReadFile(static.CalicoManifests, path.Join(dir, filename))
			if err != nil {
				return fmt.Errorf("can't find manifest %s: %w", manifestName, err)
			}

			tw := templatewriter.TemplateWriter{
				Name:     fmt.Sprintf("calico-%s-%s", dir, strings.TrimSuffix(filename, filepath.Ext(filename))),
				Template: string(contents),
				Data:     newConfig,
			}
			tryAndLog(manifestName, tw.WriteToBuffer(output))
			tryAndLog(manifestName, file.AtomicWithTarget(filepath.Join(c.manifestsDir, "calico", manifestName)).
				WithPermissions(constant.CertMode).
				Write(output.Bytes()))
		}
	}

	return nil
}

func (c *Calico) getConfig(clusterConfig *v1beta1.ClusterConfig) (*calicoClusterConfig, error) {
	ipv6AutoDetectionMethod := clusterConfig.Spec.Network.Calico.IPAutodetectionMethod
	if clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod != "" {
		ipv6AutoDetectionMethod = clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod
	}

	primaryAFIPv4 := c.primaryAddressFamily == v1beta1.PrimaryFamilyIPv4
	isDualStack := clusterConfig.Spec.Network.DualStack.Enabled
	config := calicoClusterConfig{
		MTU:                        clusterConfig.Spec.Network.Calico.MTU,
		VxlanPort:                  clusterConfig.Spec.Network.Calico.VxlanPort,
		VxlanVNI:                   clusterConfig.Spec.Network.Calico.VxlanVNI,
		EnableWireguard:            clusterConfig.Spec.Network.Calico.EnableWireguard,
		EnvVars:                    clusterConfig.Spec.Network.Calico.EnvVars,
		FlexVolumeDriverPath:       clusterConfig.Spec.Network.Calico.FlexVolumeDriverPath,
		EnableIPv4:                 isDualStack || primaryAFIPv4,
		EnableIPv6:                 isDualStack || !primaryAFIPv4,
		CalicoCNIImage:             clusterConfig.Spec.Images.Calico.CNI.URI(),
		CalicoCNIWindowsImage:      clusterConfig.Spec.Images.Calico.Windows.CNI.URI(),
		CalicoNodeImage:            clusterConfig.Spec.Images.Calico.Node.URI(),
		CalicoNodeWindowsImage:     clusterConfig.Spec.Images.Calico.Windows.Node.URI(),
		CalicoKubeControllersImage: clusterConfig.Spec.Images.Calico.KubeControllers.URI(),
		Overlay:                    clusterConfig.Spec.Network.Calico.Overlay,
		IPAutodetectionMethod:      clusterConfig.Spec.Network.Calico.IPAutodetectionMethod,
		IPV6AutodetectionMethod:    ipv6AutoDetectionMethod,
		PullPolicy:                 clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	switch clusterConfig.Spec.Network.Calico.Mode {
	case v1beta1.CalicoModeBIRD, v1beta1.CalicoModeIPIP:
		config.Mode = calicoModeBIRD
	case v1beta1.CalicoModeVXLAN:
		config.Mode = calicoModeVXLAN
	default:
		return nil, fmt.Errorf("unsupported mode: %q", clusterConfig.Spec.Network.Calico.Mode)
	}

	if isDualStack {
		config.ClusterCIDRIPv4 = clusterConfig.Spec.Network.PodCIDR
		config.ClusterCIDRIPv6 = clusterConfig.Spec.Network.DualStack.IPv6PodCIDR
	} else if primaryAFIPv4 {
		config.ClusterCIDRIPv4 = clusterConfig.Spec.Network.PodCIDR
	} else {
		config.ClusterCIDRIPv6 = clusterConfig.Spec.Network.PodCIDR
	}
	return &config, nil
}

// Stop implements [manager.Component].
func (c *Calico) Stop() error {
	if stop := c.stop; stop != nil {
		stop()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *Calico) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	c.log.Debug("reconcile method called for: Calico")
	if cfg.Spec.Network.Provider != "calico" {
		return nil
	}

	existingCNI := existingCNIProvider(c.manifestsDir)
	if existingCNI != "" && existingCNI != constant.CNIProviderCalico {
		return fmt.Errorf("cannot change CNI provider from %s to %s", existingCNI, constant.CNIProviderCalico)
	}

	calicoCRDOnce.Do(func() {
		if err := c.dumpCRDs(); err != nil {
			c.log.Errorf("error dumping Calico CRDs: %v", err)
		}
	})
	newConfig, err := c.getConfig(cfg)
	if err != nil {
		return fmt.Errorf("while generating Calico configuration: %w", err)
	}
	c.config.Set(newConfig)
	return nil
}
