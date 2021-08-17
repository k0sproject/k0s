/*
Copyright 2021 k0s authors

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
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"github.com/k0sproject/k0s/static"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
)

// Calico is the Component interface implementation to manage Calico
type Calico struct {
	clusterConf *v1beta1.ClusterConfig
	tickerDone  chan struct{}
	log         *logrus.Entry

	crdSaver manifestsSaver
	saver    manifestsSaver
}

type manifestsSaver interface {
	Save(dst string, content []byte) error
}

type calicoConfig struct {
	MTU                  int
	Mode                 string
	VxlanPort            int
	VxlanVNI             int
	ClusterCIDRIPv4      string
	ClusterCIDRIPv6      string
	EnableWireguard      bool
	WithWindowsNodes     bool
	FlexVolumeDriverPath string
	DualStack            bool

	CalicoCNIImage             string
	CalicoNodeImage            string
	CalicoKubeControllersImage string
	Overlay                    string
	IPAutodetectionMethod      string
	IPV6AutodetectionMethod    string
	PullPolicy                 string
}

// NewCalico creates new Calico reconciler component
func NewCalico(clusterConf *v1beta1.ClusterConfig, crdSaver manifestsSaver, manifestsSaver manifestsSaver) (*Calico, error) {
	log := logrus.WithFields(logrus.Fields{"component": "calico"})
	return &Calico{
		clusterConf: clusterConf,
		log:         log,
		crdSaver:    crdSaver,
		saver:       manifestsSaver,
	}, nil
}

// Init does nothing
func (c *Calico) Init() error {
	return nil
}

// Run runs the calico reconciler
func (c *Calico) Run() error {
	c.tickerDone = make(chan struct{})
	var emptyStruct struct{}

	// Write the CRD definitions only at "boot", they do not change during runtime
	crds, err := static.AssetDir("manifests/calico/CustomResourceDefinition")
	if err != nil {
		return err
	}

	for _, filename := range crds {
		manifestName := fmt.Sprintf("calico-crd-%s", filename)

		output := bytes.NewBuffer([]byte{})

		contents, err := static.Asset(fmt.Sprintf("manifests/calico/CustomResourceDefinition/%s", filename))
		if err != nil {
			return fmt.Errorf("failed to fetch crd %s: %w", filename, err)
		}

		tw := templatewriter.TemplateWriter{
			Name:     fmt.Sprintf("calico-crd-%s", strings.TrimSuffix(filename, filepath.Ext(filename))),
			Template: string(contents),
			Data:     emptyStruct,
		}
		if err := tw.WriteToBuffer(output); err != nil {
			return fmt.Errorf("failed to write calico crd manifests %s: %v", manifestName, err)
		}

		if err := c.crdSaver.Save(manifestName, output.Bytes()); err != nil {
			return fmt.Errorf("failed to save calico crd manifest %s: %v", manifestName, err)
		}
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		previousConfig := calicoConfig{}
		for {
			select {
			case <-ticker.C:
				newConfig := c.processConfigChanges(previousConfig)
				if newConfig != nil {
					previousConfig = *newConfig
				}
			case <-c.tickerDone:
				c.log.Info("calico reconciler done")
				return
			}
		}
	}()

	return nil
}

func (c *Calico) processConfigChanges(previousConfig calicoConfig) *calicoConfig {
	cfg, err := c.getConfig()
	if err != nil {
		c.log.Errorf("error calculating calico configs: %s. will retry", err.Error())
		return nil
	}
	if cfg == previousConfig {
		c.log.Infof("current cfg matches existing, not gonna do anything")
		return nil
	}

	manifestDirectories, err := static.AssetDir("manifests/calico")
	if err != nil {
		c.log.Errorf("error retrieving calico manifests: %s. will retry", err.Error())
		return nil
	}

	for _, dir := range manifestDirectories {
		// CRDs are handled separately on boot
		if dir == "CustomResourceDefinition" {
			continue
		}
		manifestPaths, err := static.AssetDir(fmt.Sprintf("manifests/calico/%s", dir))
		if err != nil {
			c.log.Errorf("error retrieving calico manifests: %s. will retry", err.Error())
			return nil
		}

		tryAndLog := func(name string, e error) {
			if e != nil {
				c.log.Errorf("failed to write manifest %s: %v, will re-try", name, e)
			}
		}

		for _, filename := range manifestPaths {
			manifestName := fmt.Sprintf("calico-%s-%s", dir, filename)
			output := bytes.NewBuffer([]byte{})
			contents, err := static.Asset(fmt.Sprintf("manifests/calico/%s/%s", dir, filename))
			if err != nil {
				return nil
			}

			tw := templatewriter.TemplateWriter{
				Name:     fmt.Sprintf("calico-%s-%s", dir, strings.TrimSuffix(filename, filepath.Ext(filename))),
				Template: string(contents),
				Data:     cfg,
			}
			tryAndLog(manifestName, tw.WriteToBuffer(output))
			tryAndLog(manifestName, c.saver.Save(manifestName, output.Bytes()))
		}
	}

	return &cfg
}

func (c *Calico) getConfig() (calicoConfig, error) {
	ipv6AutoDetectionMethod := c.clusterConf.Spec.Network.Calico.IPAutodetectionMethod
	if c.clusterConf.Spec.Network.Calico.IPv6AutodetectionMethod != "" {
		ipv6AutoDetectionMethod = c.clusterConf.Spec.Network.Calico.IPv6AutodetectionMethod
	}
	config := calicoConfig{
		MTU:                        c.clusterConf.Spec.Network.Calico.MTU,
		Mode:                       c.clusterConf.Spec.Network.Calico.Mode,
		VxlanPort:                  c.clusterConf.Spec.Network.Calico.VxlanPort,
		VxlanVNI:                   c.clusterConf.Spec.Network.Calico.VxlanVNI,
		EnableWireguard:            c.clusterConf.Spec.Network.Calico.EnableWireguard,
		FlexVolumeDriverPath:       c.clusterConf.Spec.Network.Calico.FlexVolumeDriverPath,
		DualStack:                  c.clusterConf.Spec.Network.DualStack.Enabled,
		ClusterCIDRIPv4:            c.clusterConf.Spec.Network.PodCIDR,
		ClusterCIDRIPv6:            c.clusterConf.Spec.Network.DualStack.IPv6PodCIDR,
		CalicoCNIImage:             c.clusterConf.Spec.Images.Calico.CNI.URI(),
		CalicoNodeImage:            c.clusterConf.Spec.Images.Calico.Node.URI(),
		CalicoKubeControllersImage: c.clusterConf.Spec.Images.Calico.KubeControllers.URI(),
		WithWindowsNodes:           c.clusterConf.Spec.Network.Calico.WithWindowsNodes,
		Overlay:                    c.clusterConf.Spec.Network.Calico.Overlay,
		IPAutodetectionMethod:      c.clusterConf.Spec.Network.Calico.IPAutodetectionMethod,
		IPV6AutodetectionMethod:    ipv6AutoDetectionMethod,
		PullPolicy:                 c.clusterConf.Spec.Images.DefaultPullPolicy,
	}

	return config, nil
}

// Stop stops the calico reconciler
func (c *Calico) Stop() error {
	if c.tickerDone != nil {
		close(c.tickerDone)
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *Calico) Reconcile() error {
	logrus.Debug("reconcile method called for: Calico")
	return nil
}

// Health-check interface
func (c *Calico) Healthy() error { return nil }
