/*
Copyright 2023 k0s authors

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
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/static"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// WindowsStackComponent implements the component interface
// controller unpacks windows manifests
// if windows nodes are present in the cluster
type WindowsStackComponent struct {
	log logrus.FieldLogger

	kubeClientFactory    k8sutil.ClientFactoryInterface
	k0sVars              *config.CfgVars
	saver                manifestsSaver
	prevRenderingContext windowsStackRenderingContext
}

type windowsStackRenderingContext struct {
	CNIBin          string
	CNIConf         string
	Mode            string
	KubeAPIHost     string
	KubeAPIPort     string
	IPv4ServiceCIDR string
	Nameserver      string
	NodeImage       string
}

// NewWindowsStackComponent creates new WindowsStackComponent reconciler
func NewWindowsStackComponent(k0sVars *config.CfgVars, clientFactory k8sutil.ClientFactoryInterface, saver manifestsSaver) *WindowsStackComponent {
	return &WindowsStackComponent{
		log:               logrus.WithFields(logrus.Fields{"component": "WindowsNodeController"}),
		saver:             saver,
		kubeClientFactory: clientFactory,
		k0sVars:           k0sVars,
	}
}

// Init no-op
func (n *WindowsStackComponent) Init(_ context.Context) error {
	return nil
}

// Run checks and adds labels
func (n *WindowsStackComponent) Start(ctx context.Context) error {

	go func() {
		timer := time.NewTicker(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if err := n.handleWindowsNode(ctx, n.prevRenderingContext); err != nil {
					n.log.Errorf("failed to handle windows node: %v", err)
				}
			}
		}
	}()

	return nil
}

func (n *WindowsStackComponent) handleWindowsNode(ctx context.Context, cfg windowsStackRenderingContext) error {
	client, err := n.kubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get kube client: %v", err)
	}
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/os=windows",
	})
	if err != nil {
		n.log.Errorf("failed to get node list: %v", err)
		return fmt.Errorf("failed to get node list: %v", err)
	}

	if len(nodes.Items) == 0 {
		// TODO: may be delete windows stack if it exists
		return nil
	}

	n.log.Infof("found %d windows nodes", len(nodes.Items))
	if err := n.createWindowsStack(n.prevRenderingContext); err != nil {
		n.log.Errorf("failed to create windows stack: %v", err)
		return fmt.Errorf("failed to create windows stack: %v", err)
	} else {
		n.log.Infof("successfully created windows stack")
	}
	return nil
}

func (n *WindowsStackComponent) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	if cfg.Spec.Network.Provider != "calico" {
		return fmt.Errorf("windows node controller available only for %s", constant.CNIProviderCalico)
	}

	existingCNI := existingCNIProvider(n.k0sVars.ManifestsDir)
	if existingCNI != "" && existingCNI != constant.CNIProviderCalico {
		return fmt.Errorf("windows node controller available only for %s", constant.CNIProviderCalico)
	}
	newConfig, err := n.makeCalicoRenderingContext(cfg)
	if err != nil {
		return fmt.Errorf("failed to make calico rendering context: %v", err)
	}
	if !reflect.DeepEqual(newConfig, n.prevRenderingContext) {
		n.prevRenderingContext = newConfig
	}

	return nil
}
func (n *WindowsStackComponent) makeCalicoRenderingContext(cfg *v1beta1.ClusterConfig) (windowsStackRenderingContext, error) {
	dns, err := cfg.Spec.Network.DNSAddress()
	if err != nil {
		return windowsStackRenderingContext{}, fmt.Errorf("failed to parse dns address: %v", err)
	}

	return windowsStackRenderingContext{
		// template rendering unescapes double backslashes
		CNIBin:          "c:\\\\opt\\\\cni\\\\bin",
		CNIConf:         "c:\\\\opt\\\\cni\\\\conf",
		Mode:            cfg.Spec.Network.Calico.Mode,
		KubeAPIHost:     cfg.Spec.API.Address,
		KubeAPIPort:     fmt.Sprintf("%d", cfg.Spec.API.Port),
		IPv4ServiceCIDR: cfg.Spec.Network.ServiceCIDR,
		Nameserver:      dns,
		NodeImage:       "calico/windows:v3.23.5",
	}, nil
}

// Stop no-op
func (n *WindowsStackComponent) Stop() error {
	return nil
}

// createWindowsStack creates windows stack

func (n *WindowsStackComponent) createWindowsStack(newConfig windowsStackRenderingContext) error {
	manifestDirectories, err := static.AssetDir("manifests/windows")
	if err != nil {
		return fmt.Errorf("error retrieving manifests: %v", err)
	}
	spew.Dump(manifestDirectories)
	for _, dir := range manifestDirectories {
		manifestPaths, err := static.AssetDir(fmt.Sprintf("manifests/windows/%s", dir))
		if err != nil {
			return fmt.Errorf("error retrieving manifests: %s. will retry", err.Error())
		}
		tryAndLog := func(name string, e error) {
			n.log.Debugf("writing manifest %s", name)
			if e != nil {
				n.log.Errorf("failed to write manifest %s: %v, will re-try", name, e)
			}
		}

		for _, filename := range manifestPaths {
			manifestName := fmt.Sprintf("windows-%s-%s", dir, filename)
			output := bytes.NewBuffer([]byte{})
			n.log.Debugf("Reading manifest template %s", manifestName)
			contents, err := static.Asset(fmt.Sprintf("manifests/windows/%s/%s", dir, filename))
			if err != nil {
				return fmt.Errorf("can't unpack manifest %s: %v", manifestName, err)
			}

			tw := templatewriter.TemplateWriter{
				Name:     fmt.Sprintf("windows-%s-%s", dir, strings.TrimSuffix(filename, filepath.Ext(filename))),
				Template: string(contents),
				Data:     newConfig,
			}
			tryAndLog(manifestName, tw.WriteToBuffer(output))
			tryAndLog(manifestName, n.saver.Save(manifestName, output.Bytes()))
		}
	}
	return nil
}
