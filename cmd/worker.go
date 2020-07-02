package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mirantis/mke/pkg/component"
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
		Name:   "worker",
		Usage:  "Run worker",
		Action: startWorker,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "server",
			},
		},
	}
}

func startWorker(ctx *cli.Context) error {
	serverAddress := ctx.String("server")
	if serverAddress == "" {
		return fmt.Errorf("mke worker needs the controller address as --server option")
	}

	logrus.Debugf("using server address %s", serverAddress)
	token := ctx.Args().First()
	if token == "" && !util.FileExists("/var/lib/mke/kubelet.conf") {
		return fmt.Errorf("normal kubelet kubeconfig does not exist and no join-token given. dunno how to make kubelet auth to api")
	}

	// Dump join token into kubelet-bootstrap kubeconfig
	if token != "" {
		kubeconfig, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return errors.Wrap(err, "join-token does not seem to be proper token created by 'mke token create'")
		}

		kc, err := clientcmd.Load(kubeconfig)
		kc.Clusters["mke"].Server = serverAddress

		if !util.FileExists("/var/lib/mke/pki/ca.crt") {
			os.MkdirAll(constant.CertRoot, 0755) // ignore errors in case directory exists
			ioutil.WriteFile("/var/lib/mke/pki/ca.crt", kc.Clusters["mke"].CertificateAuthorityData, 0600)
		}

		err = clientcmd.WriteToFile(*kc, constant.KubeletBootstrapConfigPath)
		if err != nil {
			return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
		}
	}

	components := make(map[string]component.Component)

	components["containerd"] = &component.ContainerD{}
	components["kubelet"] = &component.Kubelet{}

	// extract needed components
	for _,comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	// Block signals til all components are started
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	components["containerd"].Run()
	components["kubelet"].Run()

	// Wait for mke process termination
	<-c

	components["kubelet"].Stop()
	components["containerd"].Stop()

	return nil

}
