/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0s/static"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Calico is the Component interface implementation to manage Calico
type Calico struct {
	clusterConf *config.ClusterConfig
	tickerDone  chan struct{}
	log         *logrus.Entry

	saver manifestsSaver
}

type manifestsSaver interface {
	Save(dst string, content []byte) error
}

type calicoConfig struct {
	MTU             int
	Mode            string
	VxlanPort       int
	VxlanVNI        int
	ClusterCIDR     string
	EnableWireguard bool

	CalicoCNIImage             string
	CalicoFlexVolumeImage      string
	CalicoNodeImage            string
	CalicoKubeControllersImage string
}

// FsManifestsSaver saves all given manifests under the specified root dir
type FsManifestsSaver struct {
	dir string
}

// Save saves given manifest under the given path
func (f FsManifestsSaver) Save(dst string, content []byte) error {
	if err := ioutil.WriteFile(filepath.Join(f.dir, dst), content, constant.ManifestsDirMode); err != nil {
		return fmt.Errorf("can't write calico manifest configuration config map%s: %v", dst, err)
	}
	return nil
}

// NewManifestsSaver builds new filesystem manifests saver
func NewManifestsSaver() (*FsManifestsSaver, error) {
	calicoDir := path.Join(constant.DataDir, "manifests", "calico")
	err := os.MkdirAll(calicoDir, constant.ManifestsDirMode)
	if err != nil {
		return nil, err
	}
	return &FsManifestsSaver{dir: calicoDir}, nil
}

// NewCalico creates new Calico reconciler component
func NewCalico(clusterConf *config.ClusterConfig, saver manifestsSaver) (*Calico, error) {
	log := logrus.WithFields(logrus.Fields{"component": "calico"})
	return &Calico{
		clusterConf: clusterConf,
		log:         log,
		saver:       saver,
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
			return errors.Wrapf(err, "failed to fetch crd %s", filename)
		}

		tw := util.TemplateWriter{
			Name:     fmt.Sprintf("calico-crd-%s", strings.TrimSuffix(filename, filepath.Ext(filename))),
			Template: string(contents),
			Data:     emptyStruct,
		}
		if err := tw.WriteToBuffer(output); err != nil {
			return fmt.Errorf("failed to write calico crd manifests %s: %v", manifestName, err)
		}

		if err := c.saver.Save(manifestName, output.Bytes()); err != nil {
			return fmt.Errorf("failed to save calico crd manifest %s: %v", manifestName, err)
		}
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var previousConfig = calicoConfig{}
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
	config, err := c.getConfig()
	if err != nil {
		c.log.Errorf("error calculating calico configs: %s. will retry", err.Error())
		return nil
	}
	if config == previousConfig {
		c.log.Infof("current config matches existing, not gonna do anything")
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

			tw := util.TemplateWriter{
				Name:     fmt.Sprintf("calico-%s-%s", dir, strings.TrimSuffix(filename, filepath.Ext(filename))),
				Template: string(contents),
				Data:     config,
			}
			tryAndLog(manifestName, tw.WriteToBuffer(output))
			tryAndLog(manifestName, c.saver.Save(manifestName, output.Bytes()))
		}
	}

	return &config
}

func (c *Calico) getConfig() (calicoConfig, error) {
	config := calicoConfig{
		MTU:                        c.clusterConf.Spec.Network.Calico.MTU,
		Mode:                       c.clusterConf.Spec.Network.Calico.Mode,
		VxlanPort:                  c.clusterConf.Spec.Network.Calico.VxlanPort,
		VxlanVNI:                   c.clusterConf.Spec.Network.Calico.VxlanVNI,
		EnableWireguard:            c.clusterConf.Spec.Network.Calico.EnableWireguard,
		ClusterCIDR:                c.clusterConf.Spec.Network.PodCIDR,
		CalicoCNIImage:             c.clusterConf.Images.Calico.CNI.URI(),
		CalicoFlexVolumeImage:      c.clusterConf.Images.Calico.FlexVolume.URI(),
		CalicoNodeImage:            c.clusterConf.Images.Calico.Node.URI(),
		CalicoKubeControllersImage: c.clusterConf.Images.Calico.KubeControllers.URI(),
	}

	return config, nil
}

// Stop stops the calico reconciler
func (c *Calico) Stop() error {
	close(c.tickerDone)
	return nil
}

// Health-check interface
func (c *Calico) Healthy() error { return nil }
