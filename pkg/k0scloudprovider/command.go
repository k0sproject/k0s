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
	"github.com/spf13/pflag"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	"k8s.io/cloud-provider/options"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version/verflag"
)

const (
	// DefaultBindPort is the default port for the cloud controller manager
	// server. This value may be overridden by a flag at startup.
	//
	// (The constant has been aliased from k8s.io/cloud-provider, as importing
	// that package directly calls some init functions that register unwanted
	// global CLI flags. This package's init function suppresses those flags.)
	DefaultBindPort = cloudprovider.CloudControllerManagerPort
)

func init() {
	hideUndesiredGlobalFlags()
}

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
		return nil, fmt.Errorf("unable to initialize cloud provider command options: %w", err)
	}

	ccmo.KubeCloudShared.CloudProvider.Name = Name
	ccmo.Generic.ClientConnection.Kubeconfig = c.KubeConfig

	if c.BindPort != 0 {
		ccmo.SecureServing.BindPort = c.BindPort
	}

	if c.UpdateFrequency != 0 {
		ccmo.NodeStatusUpdateFrequency = metav1.Duration{Duration: c.UpdateFrequency}
	}

	cloudInitializer := func(*config.CompletedConfig) cloudprovider.Interface {
		// Returns the "k0s cloud provider" using the specified `AddressCollector`
		return newProvider(c.AddressCollector)
	}

	// K0s only supports the cloud-node controller, so only use that.
	initFuncConstructors := make(map[string]app.ControllerInitFuncConstructor)
	for _, name := range []string{names.CloudNodeController} {
		var ok bool
		initFuncConstructors[name], ok = app.DefaultInitFuncConstructors[name]
		if !ok {
			return nil, fmt.Errorf("failed to find cloud provider controller %q", name)
		}
	}

	additionalFlags := cliflag.NamedFlagSets{}

	return func(stopCh <-chan struct{}) {
		controllerAliases := names.CCMControllerAliases()
		command := app.NewCloudControllerManagerCommand(ccmo, cloudInitializer, initFuncConstructors, controllerAliases, additionalFlags, stopCh)

		// Override the commands arguments to avoid it by default using `os.Args[]`
		command.SetArgs([]string{})

		if err := command.Execute(); err != nil {
			logrus.WithError(err).Errorf("Failed to execute k0s cloud provider")
		}
	}, nil
}

// hideUndesiredGlobalFlags hides some global flags registered by k8s.io
// components, so that they aren't displayed in help texts. These flags will
// still be accepted by Cobra, i.e. they won't cause flag parsing errors, but
// that's all that can be done, since Cobra doesn't allow removing flags, nor is
// there a way to intercept and suppress their addition.
func hideUndesiredGlobalFlags() {
	var flagsToHide pflag.FlagSet
	verflag.AddFlags(&flagsToHide)
	flagsToHide.VisitAll(func(f *pflag.Flag) { f.Hidden = true })
}
