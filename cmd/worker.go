package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/component/worker"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/tools/clientcmd"
)

// WorkerCommand ...
func WorkerCommand() *cli.Command {
	return &cli.Command{
		Name:      "worker",
		Usage:     "Run worker",
		Action:    startWorker,
		ArgsUsage: "[join-token]",
	}
}

func startWorker(ctx *cli.Context) error {
	worker.KernelSetup()

	token := ctx.Args().First()
	if token == "" && !util.FileExists("/var/lib/mke/kubelet.conf") {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	var kubeletConfigClient *worker.KubeletConfigClient

	// Dump join token into kubelet-bootstrap kubeconfig if it does not already exist
	if token != "" && !util.FileExists(constant.KubeletBootstrapConfigPath) {
		kubeconfig, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return errors.Wrap(err, "join-token does not seem to be proper token created by 'mke token create'")
		}

		// Load the bootstrap kubeconfig to validate it
		clientCfg, err := clientcmd.Load(kubeconfig)
		if err != nil {
			return errors.Wrap(err, "failed to parse kubelet bootstrap auth from token")
		}

		kubeletCAPath := path.Join(constant.CertRoot, "ca.crt")
		if !util.FileExists(kubeletCAPath) {
			os.MkdirAll(constant.CertRoot, 0755) // ignore errors in case directory exists
			err = ioutil.WriteFile(kubeletCAPath, clientCfg.Clusters["mke"].CertificateAuthorityData, 0600)
			if err != nil {
				return errors.Wrap(err, "failed to write ca client cert")
			}
		}

		err = ioutil.WriteFile(constant.KubeletBootstrapConfigPath, kubeconfig, 0600)
		if err != nil {
			return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
		}

	}

	// Prefer to load client config from kubelet auth, fallback to bootstrap token auth
	clientConfigPath := constant.KubeletBootstrapConfigPath
	if util.FileExists(constant.KubeletAuthConfigPath) {
		clientConfigPath = constant.KubeletAuthConfigPath
	}

	kubeletConfigClient, err := worker.NewKubeletConfigClient(clientConfigPath)
	if err != nil {
		return err
	}

	components := make(map[string]component.Component)

	components["containerd"] = &worker.ContainerD{}
	components["kubelet"] = &worker.Kubelet{
		KubeletConfigClient: kubeletConfigClient,
	}

	// extract needed components
	for _, comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	// Set up signal handling. Use bufferend channel so we dont miss
	// signals during startup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	err = components["containerd"].Run()
	err = components["kubelet"].Run()
	if err != nil {
		logrus.Errorf("failed to start some of the worker components: %s", err.Error())
		c <- syscall.SIGTERM
	}
	// Wait for mke process termination
	<-c
	logrus.Info("Shutting down mke worker")

	components["kubelet"].Stop()
	components["containerd"].Stop()

	return nil

}
