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
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
)

type CmdOpts config.CLIOptions

var (
	apiServer        string
	cidrRange        string
	cloudProvider    bool
	clusterDNS       string
	cmdLogLevels     map[string]string
	criSocket        string
	kubeletExtraArgs string
	labels           []string
	tokenFile        string
	tokenArg         string
	workerProfile    string
)

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
			if len(args) > 0 {
				tokenArg = args[0]
			}
			c := getCmdOpts()
			c.Logging = util.MapMerge(cmdLogLevels, c.DefaultLogLevels)
			if len(tokenArg) > 0 && len(tokenFile) > 0 {
				return fmt.Errorf("You can only pass one token argument either as a CLI argument 'k0s worker [token]' or as a flag 'k0s worker --token-file [path]'")
			}

			if len(tokenFile) > 0 {
				bytes, err := ioutil.ReadFile(tokenFile)
				if err != nil {
					return err
				}
				tokenArg = string(bytes)
			}
			cmd.SilenceUsage = true
			return c.startWorker(tokenArg)
		},
	}

	cmd.Flags().StringVar(&workerProfile, "profile", "default", "worker profile to use on the node")
	cmd.Flags().StringVar(&criSocket, "cri-socket", "", "contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]")
	cmd.Flags().StringVar(&apiServer, "api-server", "", "HACK: api-server for the windows worker node")
	cmd.Flags().StringVar(&cidrRange, "cidr-range", "10.96.0.0/12", "HACK: cidr range for the windows worker node")
	cmd.Flags().StringVar(&clusterDNS, "cluster-dns", "10.96.0.10", "HACK: cluster dns for the windows worker node")
	cmd.Flags().BoolVar(&cloudProvider, "enable-cloud-provider", false, "Whether or not to enable cloud provider support in kubelet")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to the file containing token.")
	cmd.Flags().StringToStringVarP(&cmdLogLevels, "logging", "l", config.DefaultLogLevels(), "Logging Levels for the different components")
	cmd.Flags().StringSliceVarP(&labels, "labels", "", []string{}, "Node labels, list of key=value pairs")
	cmd.Flags().StringVar(&kubeletExtraArgs, "kubelet-extra-args", "", "extra args for kubelet")

	// append flags
	cmd.Flags().AddFlagSet(getPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) startWorker(token string) error {
	worker.KernelSetup()
	if token == "" && !util.FileExists(c.K0sVars.KubeletAuthConfigPath) {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	// Dump join token into kubelet-bootstrap kubeconfig if it does not already exist
	if token != "" && !util.FileExists(c.K0sVars.KubeletBootstrapConfigPath) {
		if err := worker.HandleKubeletBootstrapToken(token, c.K0sVars); err != nil {
			return err
		}
	}

	kubeletConfigClient, err := worker.LoadKubeletConfigClient(c.K0sVars)
	if err != nil {
		return err
	}

	componentManager := component.NewManager()
	if runtime.GOOS == "windows" && criSocket == "" {
		return fmt.Errorf("windows worker needs to have external CRI")
	}
	if criSocket == "" {
		componentManager.Add(&worker.ContainerD{
			LogLevel: c.Logging["containerd"],
			K0sVars:  c.K0sVars,
		})
	}

	componentManager.Add(worker.NewOCIBundleReconciler(k0sVars))

	if workerProfile == "default" && runtime.GOOS == "windows" {
		workerProfile = "default-windows"
	}

	componentManager.Add(&worker.Kubelet{
		CRISocket:           criSocket,
		EnableCloudProvider: cloudProvider,
		K0sVars:             c.K0sVars,
		KubeletConfigClient: kubeletConfigClient,
		LogLevel:            c.Logging["kubelet"],
		Profile:             workerProfile,
		Labels:              labels,
		ExtraArgs:           kubeletExtraArgs,
	})

	if runtime.GOOS == "windows" {
		if token == "" {
			return fmt.Errorf("no join-token given, which is required for windows bootstrap")
		}
		componentManager.Add(&worker.KubeProxy{
			K0sVars:   c.K0sVars,
			LogLevel:  c.Logging["kube-proxy"],
			CIDRRange: cidrRange,
		})
		componentManager.Add(&worker.CalicoInstaller{
			Token:      token,
			APIAddress: apiServer,
			CIDRRange:  cidrRange,
			ClusterDNS: clusterDNS,
		})
	}

	// extract needed components
	if err := componentManager.Init(); err != nil {
		return err
	}

	worker.KernelSetup()

	// Set up signal handling. Use buffered channel so we dont miss
	// signals during startup
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(ch)
		cancel()
	}()

	go func() {
		select {
		case <-ch:
			logrus.Info("Shutting down k0s controller")
			cancel()
		case <-ctx.Done():
			logrus.Debug("Context done in go-routine")
		}
	}()

	err = componentManager.Start(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to start some of the worker components")
		ch <- syscall.SIGTERM
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
