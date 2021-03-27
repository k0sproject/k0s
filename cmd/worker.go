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
package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/token"
)

func init() {
	workerCmd.Flags().StringVar(&workerProfile, "profile", "default", "worker profile to use on the node")
	workerCmd.Flags().StringVar(&criSocket, "cri-socket", "", "contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]")
	workerCmd.Flags().StringVar(&apiServer, "api-server", "", "HACK: api-server for the windows worker node")
	workerCmd.Flags().StringVar(&cidrRange, "cidr-range", "10.96.0.0/12", "HACK: cidr range for the windows worker node")
	workerCmd.Flags().StringVar(&clusterDNS, "cluster-dns", "10.96.0.10", "HACK: cluster dns for the windows worker node")
	workerCmd.Flags().BoolVar(&cloudProvider, "enable-cloud-provider", false, "Whether or not to enable cloud provider support in kubelet")
	workerCmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to the file containing token.")
	workerCmd.Flags().StringToStringVarP(&cmdLogLevels, "logging", "l", defaultLogLevels, "Logging Levels for the different components")
	workerCmd.Flags().StringSliceVarP(&labels, "labels", "", []string{}, "Node labels, list of key=value pairs")
	workerCmd.Flags().StringVar(&kubeletExtraArgs, "kubelet-extra-args", "", "extra args for kubelet")

	addPersistentFlags(workerCmd)
	controllerCmd.Flags().AddFlagSet(workerCmd.Flags())
	installWorkerCmd.Flags().AddFlagSet(workerCmd.Flags())
}

var (
	apiServer        string
	cidrRange        string
	cloudProvider    bool
	clusterDNS       string
	criSocket        string
	labels           []string
	tokenArg         string
	tokenFile        string
	workerProfile    string
	kubeletExtraArgs string

	workerCmd = &cobra.Command{
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
			return startWorker(tokenArg)
		},
	}
)

func startWorker(token string) error {

	worker.KernelSetup()
	if token == "" && !util.FileExists(k0sVars.KubeletAuthConfigPath) {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	// Dump join token into kubelet-bootstrap kubeconfig if it does not already exist
	if token != "" && !util.FileExists(k0sVars.KubeletBootstrapConfigPath) {
		if err := handleKubeletBootstrapToken(token, k0sVars); err != nil {
			return err
		}
	}

	kubeletConfigClient, err := loadKubeletConfigClient(k0sVars)
	if err != nil {
		return err
	}

	componentManager := component.NewManager()
	if runtime.GOOS == "windows" && criSocket == "" {
		return fmt.Errorf("windows worker needs to have external CRI")
	}
	if criSocket == "" {
		componentManager.Add(&worker.ContainerD{
			LogLevel: logging["containerd"],
			K0sVars:  k0sVars,
		})
	}

	componentManager.Add(worker.NewOCIBundleReconciler(k0sVars))

	if workerProfile == "default" && runtime.GOOS == "windows" {
		workerProfile = "default-windows"
	}

	componentManager.Add(&worker.Kubelet{
		CRISocket:           criSocket,
		EnableCloudProvider: cloudProvider,
		K0sVars:             k0sVars,
		KubeletConfigClient: kubeletConfigClient,
		LogLevel:            logging["kubelet"],
		Profile:             workerProfile,
		Labels:              labels,
		ExtraArgs:           kubeletExtraArgs,
	})

	if runtime.GOOS == "windows" {
		if token == "" {
			return fmt.Errorf("no join-token given, which is required for windows bootstrap")
		}
		componentManager.Add(&worker.KubeProxy{
			K0sVars:   k0sVars,
			LogLevel:  logging["kube-proxy"],
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		cancel()
	}()

	go func() {
		select {
		case <-c:
			logrus.Info("Shutting down k0s controller")
			cancel()
		case <-ctx.Done():
			logrus.Debug("Context done in go-routine")
		}
	}()

	err = componentManager.Start(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to start some of the worker components")
		c <- syscall.SIGTERM
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

func loadKubeletConfigClient(k0svars constant.CfgVars) (*worker.KubeletConfigClient, error) {
	var kubeletConfigClient *worker.KubeletConfigClient
	// Prefer to load client config from kubelet auth, fallback to bootstrap token auth
	clientConfigPath := k0svars.KubeletBootstrapConfigPath
	if util.FileExists(k0svars.KubeletAuthConfigPath) {
		clientConfigPath = k0svars.KubeletAuthConfigPath
	}

	kubeletConfigClient, err := worker.NewKubeletConfigClient(clientConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start kubelet config client: %v", err)
	}
	return kubeletConfigClient, nil
}

func handleKubeletBootstrapToken(encodedToken string, k0sVars constant.CfgVars) error {
	kubeconfig, err := token.DecodeJoinToken(encodedToken)
	if err != nil {
		return errors.Wrap(err, "failed to decode token")
	}

	// Load the bootstrap kubeconfig to validate it
	clientCfg, err := clientcmd.Load(kubeconfig)

	if err != nil {
		return errors.Wrap(err, "failed to parse kubelet bootstrap auth from token")
	}

	kubeletCAPath := path.Join(k0sVars.CertRootDir, "ca.crt")
	if !util.FileExists(kubeletCAPath) {
		if err := util.InitDirectory(k0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to initialize dir: %v", k0sVars.CertRootDir))
		}
		err = ioutil.WriteFile(kubeletCAPath, clientCfg.Clusters["k0s"].CertificateAuthorityData, constant.CertMode)
		if err != nil {
			return errors.Wrap(err, "failed to write ca client cert")
		}
	}

	err = ioutil.WriteFile(k0sVars.KubeletBootstrapConfigPath, kubeconfig, constant.CertSecureMode)
	if err != nil {
		return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
	}

	return nil
}
