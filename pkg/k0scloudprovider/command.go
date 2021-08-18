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
package k0scloudprovider

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

type Command func(stopCh <-chan struct{})

type Config struct {
	AddressCollector AddressCollector
	KubeConfig       string
	BindPort         int
	UpdateFrequency  time.Duration
}

// NewCommand creates a new k0s-cloud-provider based on a configuration.
// The command itself is a specialization of the sample code available from
// `k8s.io/cloud-provider/app`
func NewCommand(c Config) (Command, error) {
	ccmo, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		return nil, fmt.Errorf("unable to initialize command options: %w", err)
	}

	ccmo.KubeCloudShared.CloudProvider.Name = Name
	ccmo.Kubeconfig = c.KubeConfig

	if c.BindPort != 0 {
		ccmo.SecureServing.BindPort = c.BindPort
	}

	if c.UpdateFrequency != 0 {
		ccmo.NodeStatusUpdateFrequency = metav1.Duration{Duration: c.UpdateFrequency}
	}

	controllerList := []string{"cloud-node", "cloud-node-lifecycle", "service", "route"}
	disabledControllerList := []string{"service", "route"}

	ccmc, err := ccmo.Config(controllerList, disabledControllerList)
	if err != nil {
		return nil, fmt.Errorf("unable to create k0s-cloud-provider configuration: %w", err)
	}

	// Builds the provider using the specified `AddressCollector`

	cloud := NewProvider(c.AddressCollector)
	if err != nil {
		return nil, fmt.Errorf("unable to create k0s-cloud-provider: %w", err)
	}

	return func(stopCh <-chan struct{}) {
		cloud.Initialize(ccmc.ClientBuilder, stopCh)

		controllerInitializers := app.ConstructControllerInitializers(app.DefaultInitFuncConstructors, ccmc.Complete(), cloud)
		for _, disabledController := range disabledControllerList {
			delete(controllerInitializers, disabledController)
		}

		// Override the commands arguments to avoid it by default using `os.Args[]`
		fss := cliflag.NamedFlagSets{}
		command := app.NewCloudControllerManagerCommand(ccmo, cloudInitializer, app.DefaultInitFuncConstructors, fss, stopCh)
		// command := app.NewCloudControllerManagerCommand(ccmo, ccmc, controllerInitializers)
		command.SetArgs([]string{})

		if err := command.Execute(); err != nil {
			logrus.Errorf("unable to execute command: %v", err)
		}
	}, nil
}

func cloudInitializer(config *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}
	return cloud
}
