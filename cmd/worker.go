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
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/token"
	"github.com/k0sproject/k0s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	workerCmd.Flags().StringVar(&workerProfile, "profile", "default", "worker profile to use on the node")
	workerCmd.Flags().StringVar(&criSocket, "cri-socket", "", "contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]")
	workerCmd.Flags().BoolVar(&cloudProvider, "enable-cloud-provider", false, "Whether or not to enable cloud provider support in kubelet")
	workerCmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to the file containing token.")
}

var (
	workerProfile string
	tokenArg      string
	tokenFile     string
	criSocket     string
	cloudProvider bool

	workerCmd = &cobra.Command{
		Use:   "worker [join-token]",
		Short: "Run worker",
		Example: `	Command to add worker node to the master node:
	CLI agument:
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
	if token == "" && !util.FileExists(constant.KubeletAuthConfigPath) {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	// Dump join token into kubelet-bootstrap kubeconfig if it does not already exist
	if token != "" && !util.FileExists(constant.KubeletBootstrapConfigPath) {
		if err := handleKubeletBootstrapToken(token); err != nil {
			return err
		}
	}

	kubeletConfigClient, err := loadKubeletConfigClient()
	if err != nil {
		return err
	}

	componentManager := component.NewManager()
	if criSocket == "" {
		componentManager.Add(&worker.ContainerD{
			LogLevel: logging["containerd"],
		})
	}

	componentManager.Add(&worker.Kubelet{
		KubeletConfigClient: kubeletConfigClient,
		Profile:             workerProfile,
		CRISocket:           criSocket,
		LogLevel:            logging["kubelet"],
		EnableCloudProvider: cloudProvider,
	})

	// extract needed components
	if err := componentManager.Init(); err != nil {
		return err
	}

	// Set up signal handling. Use bufferend channel so we dont miss
	// signals during startup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	err = componentManager.Start()
	if err != nil {
		logrus.Errorf("failed to start some of the worker components: %s", err.Error())
		c <- syscall.SIGTERM
	}
	// Wait for k0s process termination
	<-c
	logrus.Info("Shutting down k0s worker")

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.Errorf("error while stoping component manager %s", err)
	}
	return nil

}

func loadKubeletConfigClient() (*worker.KubeletConfigClient, error) {
	var kubeletConfigClient *worker.KubeletConfigClient
	// Prefer to load client config from kubelet auth, fallback to bootstrap token auth
	clientConfigPath := constant.KubeletBootstrapConfigPath
	if util.FileExists(constant.KubeletAuthConfigPath) {
		clientConfigPath = constant.KubeletAuthConfigPath
	}

	kubeletConfigClient, err := worker.NewKubeletConfigClient(clientConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start kubelet config client: %v", err)
	}
	return kubeletConfigClient, nil
}

func handleKubeletBootstrapToken(encodedToken string) error {
	kubeconfig, err := token.JoinDecode(encodedToken)
	if err != nil {
		return errors.Wrap(err, "failed to decode token")
	}

	// Load the bootstrap kubeconfig to validate it
	clientCfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "failed to parse kubelet bootstrap auth from token")
	}

	kubeletCAPath := path.Join(constant.CertRootDir, "ca.crt")
	if !util.FileExists(kubeletCAPath) {
		if err := util.InitDirectory(constant.CertRootDir, constant.CertRootDirMode); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to initialize dir: %v", constant.CertRootDir))
		}
		err = ioutil.WriteFile(kubeletCAPath, clientCfg.Clusters["k0s"].CertificateAuthorityData, constant.CertMode)
		if err != nil {
			return errors.Wrap(err, "failed to write ca client cert")
		}
	}

	err = ioutil.WriteFile(constant.KubeletBootstrapConfigPath, kubeconfig, constant.CertSecureMode)
	if err != nil {
		return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
	}

	return nil
}
