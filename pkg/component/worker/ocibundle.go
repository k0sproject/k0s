package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go"
	"github.com/containerd/containerd"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	k0sVars constant.CfgVars
	log     *logrus.Entry
}

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars constant.CfgVars) *OCIBundleReconciler {
	return &OCIBundleReconciler{
		k0sVars: vars,
		log:     logrus.WithField("component", "OCIBundleReconciler"),
	}
}

func (a *OCIBundleReconciler) Init() error {
	return dir.Init(a.k0sVars.OCIBundleDir, constant.ManifestsDirMode)
}

func (a *OCIBundleReconciler) Run() error {
	files, err := os.ReadDir(a.k0sVars.OCIBundleDir)
	if err != nil {
		return fmt.Errorf("can't read bundles directory")
	}
	if len(files) == 0 {
		return nil
	}
	var client *containerd.Client
	sock := filepath.Join(a.k0sVars.RunDir, "containerd.sock")
	err = retry.Do(func() error {
		client, err = containerd.New(sock, containerd.WithDefaultNamespace("k8s.io"))
		if err != nil {
			logrus.WithError(err).Errorf("can't connect to containerd socket %s", sock)
			return err
		}
		_, err := client.ListImages(context.Background())
		if err != nil {
			logrus.WithError(err).Errorf("can't use containerd client")
			return err
		}
		return nil
	}, retry.Delay(time.Second*5))
	if err != nil {
		return fmt.Errorf("can't connect to containerd socket %s: %v", sock, err)
	}
	defer client.Close()

	for _, file := range files {
		if err := a.unpackBundle(client, a.k0sVars.OCIBundleDir+"/"+file.Name()); err != nil {
			logrus.WithError(err).Errorf("can't unpack bundle %s", file.Name())
			return fmt.Errorf("can't unpack bundle %s: %w", file.Name(), err)
		}
	}
	return nil
}

func (a OCIBundleReconciler) unpackBundle(client *containerd.Client, bundlePath string) error {
	r, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("can't open bundle file %s: %v", bundlePath, err)
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

func (a *OCIBundleReconciler) Stop() error {
	return nil
}

func (a *OCIBundleReconciler) Reconcile() error {
	logrus.Debug("reconcile method called for: OCIBundleReconciler")
	return nil
}
func (a *OCIBundleReconciler) Healthy() error { return nil }
