package server

import (
	"fmt"
	"github.com/Mirantis/mke/static"
	"path/filepath"
	"strings"
	"time"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	k8sutil "github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Calico is the Component interface implementation to manage Calico
type Calico struct {
	client      kubernetes.Interface
	clusterSpec *config.ClusterSpec
	tickerDone  chan struct{}
	log         *logrus.Entry
}

type calicoConfig struct {
	MTU         int
	Mode        string
	VxlanPort   int
	VxlanVNI    int
	ClusterCIDR string
}

// NewCalico creates new Calico reconciler component
func NewCalico(clusterSpec *config.ClusterSpec) (*Calico, error) {
	client, err := k8sutil.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return nil, err
	}
	log := logrus.WithFields(logrus.Fields{"component": "calico"})
	return &Calico{
		client:      client,
		clusterSpec: clusterSpec,
		log:         log,
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
		contents, err := static.Asset(fmt.Sprintf("manifests/calico/CustomResourceDefinition/%s", filename))

		if err != nil {
			return errors.Wrapf(err, "failed to fetch crd %s", filename)
		}

		tw := util.TemplateWriter{
			Name:     fmt.Sprintf("calico-crd-%s", strings.TrimSuffix(filename, filepath.Ext(filename))),
			Template: string(contents),
			Data:     emptyStruct,
			Path:     filepath.Join(constant.DataDir, "manifests", "calico", fmt.Sprintf("calico-crd-%s", filename)),
		}
		err = tw.Write()
		if err != nil {
			return errors.Wrap(err, "failed to write calico crd manifests")
		}
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var previousConfig = calicoConfig{}
		for {
			select {
			case <-ticker.C:
				newConfig := c.work(previousConfig)
				if newConfig != nil {
					previousConfig = *newConfig
				}
			case <-c.tickerDone:
				c.log.Info("coredns reconciler done")
				return
			}
		}
	}()

	return nil
}

func (c *Calico) work(previousConfig calicoConfig) *calicoConfig {
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

		for _, filename := range manifestPaths {
			contents, err := static.Asset(fmt.Sprintf("manifests/calico/%s/%s", dir, filename))
			if err != nil {
				return nil
			}

			tw := util.TemplateWriter{
				Name:     fmt.Sprintf("calico-%s-%s", dir, strings.TrimSuffix(filename, filepath.Ext(filename))),
				Template: string(contents),
				Data:     config,
				Path:     filepath.Join(constant.DataDir, "manifests", "calico", fmt.Sprintf("calico-%s-%s", dir, filename)),
			}
			err = tw.Write()
			if err != nil {
				c.log.Errorf("error writing calico manifest: %s. will retry", err.Error())
				return nil
			}
		}
	}

	return &config
}

func (c *Calico) getConfig() (calicoConfig, error) {
	config := calicoConfig{
		MTU:         c.clusterSpec.Network.Calico.MTU,
		Mode:        c.clusterSpec.Network.Calico.Mode,
		VxlanPort:   c.clusterSpec.Network.Calico.VxlanPort,
		VxlanVNI:    c.clusterSpec.Network.Calico.VxlanVNI,
		ClusterCIDR: c.clusterSpec.Network.PodCIDR,
	}

	return config, nil
}

// Stop stops the calico reconciler
func (c *Calico) Stop() error {
	close(c.tickerDone)
	return nil
}
