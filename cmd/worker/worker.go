/*
Copyright 2022 k0s authors

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
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

var ignorePreFlightChecks bool

func NewWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker [join-token]",
		Short: "Run worker",
		Example: `	Command to add worker node to the master node:
	CLI argument:
	$ k0s worker [token]

	or CLI flag:
	$ k0s worker --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetLevel(logrus.InfoLevel)
			logrus.SetOutput(os.Stdout)

			c := CmdOpts(config.GetCmdOpts())
			if len(args) > 0 {
				c.TokenArg = args[0]
			}

			c.Logging = stringmap.Merge(c.CmdLogLevels, c.DefaultLogLevels)
			if len(c.TokenArg) > 0 && len(c.TokenFile) > 0 {
				return fmt.Errorf("you can only pass one token argument either as a CLI argument 'k0s worker [token]' or as a flag 'k0s worker --token-file [path]'")
			}

			if len(c.TokenFile) > 0 {
				bytes, err := os.ReadFile(c.TokenFile)
				if err != nil {
					return err
				}
				c.TokenArg = string(bytes)
			}
			cmd.SilenceUsage = true

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

			return c.StartWorker(ctx)
		},
	}

	// append flags
	cmd.Flags().BoolVar(&ignorePreFlightChecks, "ignore-pre-flight-checks", false, "continue even if pre-flight checks fail")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.PersistentFlags().AddFlagSet(config.GetWorkerFlags())
	return cmd
}

// StartWorker starts the worker components based on the CmdOpts config
func (c *CmdOpts) StartWorker(ctx context.Context) error {
	if c.TokenArg == "" && !file.Exists(c.K0sVars.KubeletAuthConfigPath) {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	// Dump join token into kubelet-bootstrap kubeconfig if it does not already exist
	if c.TokenArg != "" && !file.Exists(c.K0sVars.KubeletBootstrapConfigPath) {
		if err := worker.HandleKubeletBootstrapToken(c.TokenArg, c.K0sVars); err != nil {
			return err
		}
	}

	kubeletConfigClient, err := worker.LoadKubeletConfigClient(c.K0sVars)
	if err != nil {
		return err
	}

	componentManager := component.NewManager()
	if runtime.GOOS == "windows" && c.CriSocket == "" {
		return fmt.Errorf("windows worker needs to have external CRI")
	}
	if c.CriSocket == "" {
		componentManager.Add(ctx, &worker.ContainerD{
			LogLevel: c.Logging["containerd"],
			K0sVars:  c.K0sVars,
		})
	}

	componentManager.Add(ctx, worker.NewOCIBundleReconciler(c.K0sVars))
	if c.WorkerProfile == "default" && runtime.GOOS == "windows" {
		c.WorkerProfile = "default-windows"
	}

	componentManager.Add(ctx, &worker.Kubelet{
		CRISocket:           c.CriSocket,
		EnableCloudProvider: c.CloudProvider,
		K0sVars:             c.K0sVars,
		KubeletConfigClient: kubeletConfigClient,
		LogLevel:            c.Logging["kubelet"],
		Profile:             c.WorkerProfile,
		Labels:              c.Labels,
		Taints:              c.Taints,
		ExtraArgs:           c.KubeletExtraArgs,
	})

	if runtime.GOOS == "windows" {
		if c.TokenArg == "" {
			return fmt.Errorf("no join-token given, which is required for windows bootstrap")
		}
		componentManager.Add(ctx, &worker.KubeProxy{
			K0sVars:   c.K0sVars,
			LogLevel:  c.Logging["kube-proxy"],
			CIDRRange: c.CIDRRange,
		})
		componentManager.Add(ctx, &worker.CalicoInstaller{
			Token:      c.TokenArg,
			APIAddress: c.APIServer,
			CIDRRange:  c.CIDRRange,
			ClusterDNS: c.ClusterDNS,
		})
	}

	if !c.SingleNode && !c.EnableWorker {
		componentManager.Add(ctx, &status.Status{
			StatusInformation: install.K0sStatus{
				Pid:           os.Getpid(),
				Role:          "worker",
				Args:          os.Args,
				Version:       build.Version,
				Workloads:     true,
				SingleNode:    false,
				K0sVars:       c.K0sVars,
				ClusterConfig: c.ClusterConfig,
			},
			Socket: config.StatusSocket,
		})
	}
	// extract needed components
	if err := componentManager.Init(ctx); err != nil {
		return err
	}

	worker.KernelSetup()
	err = componentManager.Start(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to start some of the worker components")
		return err
	}
	// Wait for k0s process termination
	<-ctx.Done()
	logrus.Info("Shutting down k0s worker")

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.WithError(err).Error("error while stoping component manager")
	}
	return nil
}
