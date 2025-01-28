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

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/k0sproject/k0s/internal/pkg/flags"
	internallog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/pkg/component/iptables"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/component/worker/nllb"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/node"

	apitypes "k8s.io/apimachinery/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Command config.CLIOptions

func NewWorkerCmd() *cobra.Command {
	var ignorePreFlightChecks bool

	cmd := &cobra.Command{
		Use:   "worker [join-token]",
		Short: "Run worker",
		Example: `	Command to add worker node to the master node:
	CLI argument:
	$ k0s worker [token]

	or CLI flag:
	$ k0s worker --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag`,
		Args: cobra.MaximumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetOutput(cmd.OutOrStdout())
			internallog.SetInfoLevel()
			return config.CallParentPersistentPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			c := (*Command)(opts)
			if len(args) > 0 {
				c.TokenArg = args[0]
			}

			if c.TokenArg != "" && c.TokenFile != "" {
				return errors.New("you can only pass one token argument either as a CLI argument 'k0s worker [token]' or as a flag 'k0s worker --token-file [path]'")
			}

			nodeName, kubeletExtraArgs, err := GetNodeName(&c.WorkerOptions)
			if err != nil {
				return fmt.Errorf("failed to determine node name: %w", err)
			}

			if err := (&sysinfo.K0sSysinfoSpec{
				ControllerRoleEnabled: false,
				WorkerRoleEnabled:     true,
				DataDir:               c.K0sVars.DataDir,
			}).RunPreFlightChecks(ignorePreFlightChecks); !ignorePreFlightChecks && err != nil {
				return err
			}

			// Set up signal handling
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			return c.Start(ctx, nodeName, kubeletExtraArgs)
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetWorkerFlags())
	flags.BoolVar(&ignorePreFlightChecks, "ignore-pre-flight-checks", false, "continue even if pre-flight checks fail")

	return cmd
}

func GetNodeName(opts *config.WorkerOptions) (apitypes.NodeName, stringmap.StringMap, error) {
	// The node name used during bootstrapping needs to match the node name
	// selected by kubelet. Otherwise, kubelet will have problems interacting
	// with a Node object that doesn't match the name in the certificates.
	// https://kubernetes.io/docs/reference/access-authn-authz/node/

	// Kubelet still has some deprecated support for cloud providers, which may
	// completely bypass the "standard" node name detection as it's done here.
	// K0s only supports external cloud providers, which seems to be a dead code
	// path anyways in kubelet. So it's safe to assume that the following code
	// exactly matches the behavior of kubelet.

	kubeletExtraArgs := flags.Split(opts.KubeletExtraArgs)
	nodeName, err := node.GetNodeName(kubeletExtraArgs["--hostname-override"])
	if err != nil {
		return "", nil, err
	}
	return nodeName, kubeletExtraArgs, nil
}

// Start starts the worker components based on the given [config.CLIOptions].
func (c *Command) Start(ctx context.Context, nodeName apitypes.NodeName, kubeletExtraArgs stringmap.StringMap) error {
	if err := worker.BootstrapKubeletKubeconfig(ctx, c.K0sVars, nodeName, &c.WorkerOptions); err != nil {
		return err
	}

	kubeletKubeconfigPath := c.K0sVars.KubeletAuthConfigPath
	workerConfig, err := workerconfig.LoadProfile(
		ctx,
		kubernetes.KubeconfigFromFile(kubeletKubeconfigPath),
		c.K0sVars.DataDir,
		c.WorkerProfile,
	)
	if err != nil {
		return err
	}

	componentManager := manager.New(prober.DefaultProber)

	var staticPods worker.StaticPods

	if workerConfig.NodeLocalLoadBalancing.IsEnabled() {
		if c.SingleNode {
			return errors.New("node-local load balancing cannot be used in a single-node cluster")
		}

		sp := worker.NewStaticPods()
		reconciler, err := nllb.NewReconciler(c.K0sVars, sp, c.WorkerProfile, *workerConfig.DeepCopy())
		if err != nil {
			return fmt.Errorf("failed to create node-local load balancer reconciler: %w", err)
		}
		kubeletKubeconfigPath = reconciler.GetKubeletKubeconfigPath()
		staticPods = sp

		componentManager.Add(ctx, sp)
		componentManager.Add(ctx, reconciler)
	}

	if c.CriSocket == "" {
		componentManager.Add(ctx, containerd.NewComponent(c.LogLevels.Containerd, c.K0sVars, workerConfig))
		componentManager.Add(ctx, worker.NewOCIBundleReconciler(c.K0sVars))
	}

	if c.WorkerProfile == "default" && runtime.GOOS == "windows" {
		c.WorkerProfile = "default-windows"
	}

	if !c.DisableIPTables {
		componentManager.Add(ctx, &iptables.Component{
			IPTablesMode: c.WorkerOptions.IPTablesMode,
			BinDir:       c.K0sVars.BinDir,
		})
	}
	componentManager.Add(ctx,
		&worker.Kubelet{
			NodeName:            nodeName,
			CRISocket:           c.CriSocket,
			EnableCloudProvider: c.CloudProvider,
			K0sVars:             c.K0sVars,
			StaticPods:          staticPods,
			Kubeconfig:          kubeletKubeconfigPath,
			Configuration:       *workerConfig.KubeletConfiguration.DeepCopy(),
			LogLevel:            c.LogLevels.Kubelet,
			Labels:              c.Labels,
			Taints:              c.Taints,
			ExtraArgs:           kubeletExtraArgs,
			DualStackEnabled:    workerConfig.DualStackEnabled,
		})

	certManager := worker.NewCertificateManager(kubeletKubeconfigPath)

	addPlatformSpecificComponents(ctx, componentManager, c.K0sVars, &c.ControllerOptions, certManager)

	// extract needed components
	if err := componentManager.Init(ctx); err != nil {
		return err
	}

	worker.KernelSetup()

	err = componentManager.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start worker components: %w", err)
	}
	// Wait for k0s process termination
	<-ctx.Done()
	logrus.Info("Shutting down k0s worker")

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.WithError(err).Error("error while stopping component manager")
	}
	return nil
}
