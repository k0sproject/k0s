/*
Copyright 2020 k0s authors

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
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/k0sproject/k0s/static"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*Calico)(nil)
var _ manager.Reconciler = (*Calico)(nil)

var calicoCRDOnce sync.Once

// Calico is the Component interface implementation to manage Calico
type Calico struct {
	log logrus.FieldLogger

	crdSaver   manifestsSaver
	saver      manifestsSaver
	prevConfig calicoConfig
	k0sVars    *config.CfgVars
}

type calicoMode string

const (
	calicoModeBIRD  calicoMode = "bird"
	calicoModeVXLAN calicoMode = "vxlan"
)

type manifestsSaver interface {
	Save(dst string, content []byte) error
}

type calicoConfig struct {
	MTU                  int
	Mode                 calicoMode
	VxlanPort            int
	VxlanVNI             int
	ClusterCIDRIPv4      string
	ClusterCIDRIPv6      string
	EnableWireguard      bool
	FlexVolumeDriverPath string
	DualStack            bool
	EnvVars              map[string]string

	CalicoCNIImage             string
	CalicoNodeImage            string
	CalicoKubeControllersImage string
	Overlay                    string
	IPAutodetectionMethod      string
	IPV6AutodetectionMethod    string
	PullPolicy                 string
}

// NewCalico creates new Calico reconciler component
func NewCalico(k0sVars *config.CfgVars, crdSaver manifestsSaver, manifestsSaver manifestsSaver) *Calico {
	return &Calico{
		log: logrus.WithFields(logrus.Fields{"component": "calico"}),

		crdSaver:   crdSaver,
		saver:      manifestsSaver,
		prevConfig: calicoConfig{},
		k0sVars:    k0sVars,
	}
}

// Init does nothing
func (c *Calico) Init(_ context.Context) error {
	return nil
}

// Run nothing really running, all logic based on reactive reconcile
func (c *Calico) Start(_ context.Context) error {
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
		manifestName := fmt.Sprintf("calico-crd-%s", filename)

		output := bytes.NewBuffer([]byte{})

		contents, err := fs.ReadFile(static.CalicoManifests, path.Join("CustomResourceDefinition", filename))
		if err != nil {
			return fmt.Errorf("failed to fetch crd %s: %w", filename, err)
		}

		tw := templatewriter.TemplateWriter{
			Name:     fmt.Sprintf("calico-crd-%s", strings.TrimSuffix(filename, filepath.Ext(filename))),
			Template: string(contents),
			Data:     emptyStruct,
		}
		if err := tw.WriteToBuffer(output); err != nil {
			return fmt.Errorf("failed to write calico crd manifests %s: %w", manifestName, err)
		}

		if err := c.crdSaver.Save(manifestName, output.Bytes()); err != nil {
			return fmt.Errorf("failed to save calico crd manifest %s: %w", manifestName, err)
		}
	}

	return nil
}

func (c *Calico) processConfigChanges(newConfig calicoConfig) error {
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
			tryAndLog(manifestName, c.saver.Save(manifestName, output.Bytes()))
		}
	}

	return nil
}

func (c *Calico) getConfig(clusterConfig *v1beta1.ClusterConfig) (calicoConfig, error) {
	ipv6AutoDetectionMethod := clusterConfig.Spec.Network.Calico.IPAutodetectionMethod
	if clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod != "" {
		ipv6AutoDetectionMethod = clusterConfig.Spec.Network.Calico.IPv6AutodetectionMethod
	}
	config := calicoConfig{
		MTU:                        clusterConfig.Spec.Network.Calico.MTU,
		VxlanPort:                  clusterConfig.Spec.Network.Calico.VxlanPort,
		VxlanVNI:                   clusterConfig.Spec.Network.Calico.VxlanVNI,
		EnableWireguard:            clusterConfig.Spec.Network.Calico.EnableWireguard,
		EnvVars:                    clusterConfig.Spec.Network.Calico.EnvVars,
		FlexVolumeDriverPath:       clusterConfig.Spec.Network.Calico.FlexVolumeDriverPath,
		DualStack:                  clusterConfig.Spec.Network.DualStack.Enabled,
		ClusterCIDRIPv4:            clusterConfig.Spec.Network.PodCIDR,
		ClusterCIDRIPv6:            clusterConfig.Spec.Network.DualStack.IPv6PodCIDR,
		CalicoCNIImage:             clusterConfig.Spec.Images.Calico.CNI.URI(),
		CalicoNodeImage:            clusterConfig.Spec.Images.Calico.Node.URI(),
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
		return config, fmt.Errorf("unsupported mode: %q", clusterConfig.Spec.Network.Calico.Mode)
	}

	return config, nil
}

// Stop stops the calico reconciler
func (c *Calico) Stop() error {
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (c *Calico) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	c.log.Debug("reconcile method called for: Calico")
	if cfg.Spec.Network.Provider != "calico" {
		return nil
	}

	existingCNI := existingCNIProvider(c.k0sVars.ManifestsDir)
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
	if !reflect.DeepEqual(newConfig, c.prevConfig) {
		if err := c.processConfigChanges(newConfig); err != nil {
			c.log.Warnf("failed to process config changes: %v", err)
		}
		c.prevConfig = newConfig
	}
	return nil
}
