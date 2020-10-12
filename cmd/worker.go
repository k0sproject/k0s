package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/component/worker"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/token"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/tools/clientcmd"
)

// WorkerCommand ...
func WorkerCommand() *cli.Command {
	return &cli.Command{
		Name:   "worker",
		Usage:  "Run worker",
		Action: startWorker,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Value: "default",
				Usage: "worker profile to use on the node",
			},
		},
		ArgsUsage: "[join-token]",
	}
}

func startWorker(ctx *cli.Context) error {
	worker.KernelSetup()

	token := ctx.Args().First()
	if token == "" && !util.FileExists(path.Join(constant.DataDir, "kubelet.conf")) {
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
	componentManager.Add(&worker.ContainerD{})
	componentManager.Add(&worker.Kubelet{
		KubeletConfigClient: kubeletConfigClient,
		Profile:             ctx.String("profile"),
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
	// Wait for mke process termination
	<-c
	logrus.Info("Shutting down mke worker")

	return componentManager.Stop()

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
		return nil, err
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

	kubeletCAPath := path.Join(constant.CertRoot, "ca.crt")
	if !util.FileExists(kubeletCAPath) {
		if err := util.InitDirectory(constant.CertRoot, constant.CertRootMode); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to initialize dir: %v", constant.CertRoot))
		}
		err = ioutil.WriteFile(kubeletCAPath, clientCfg.Clusters["mke"].CertificateAuthorityData, constant.CertRootSecureMode)
		if err != nil {
			return errors.Wrap(err, "failed to write ca client cert")
		}
	}

	err = ioutil.WriteFile(constant.KubeletBootstrapConfigPath, kubeconfig, constant.CertRootSecureMode)
	if err != nil {
		return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
	}

	return nil
}
