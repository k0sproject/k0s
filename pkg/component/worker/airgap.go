package worker

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/containerd/containerd"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

// AirgapReconciler tries to import OCI bundle into the running containerd instance
type AirgapReconciler struct {
	bundlePath string
	k0sVars    constant.CfgVars
	log        *logrus.Entry
}

// NewAirgapReconciler builds new reconciler
func NewAirgapReconciler(path string, vars constant.CfgVars) *AirgapReconciler {
	return &AirgapReconciler{
		bundlePath: path,
		k0sVars:    vars,
		log:        logrus.WithField("component", "airgapReconciler"),
	}
}

func (a *AirgapReconciler) Init() error {
	return nil
}

func (a AirgapReconciler) unpackBundle() error {

	sock := filepath.Join(a.k0sVars.RunDir, "containerd.sock")
	client, err := containerd.New(sock, containerd.WithDefaultNamespace("k8s.io"))

	if err != nil {
		return fmt.Errorf("can't connect to containerd socket %s: %v", sock, err)
	}
	defer client.Close()
	r, err := os.Open(a.bundlePath)
	if err != nil {
		return fmt.Errorf("can't open bundle file %s: %v", a.bundlePath, err)
	}
	defer r.Close()
	images, err := client.Import(context.Background(), r)
	if err != nil {
		return fmt.Errorf("can't import bundle: %v", err)
	}
	for _, i := range images {
		logrus.Infof("Imported image %s", i.Name)
	}
	return nil
}

func (a *AirgapReconciler) Run() error {
	return retry.Do(func() error {
		if err := a.unpackBundle(); err != nil {
			a.log.WithError(err).Warn("can't unpack OCI bundle for airgap install")
			return err
		}
		return nil
	}, retry.Delay(time.Second*5),
		retry.Attempts(50))
}

func (a *AirgapReconciler) Stop() error {
	return nil
}

func (a *AirgapReconciler) Healthy() error {
	return nil
}
