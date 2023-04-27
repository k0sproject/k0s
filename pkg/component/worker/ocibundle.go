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
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/platforms"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	k0sVars *config.CfgVars
	log     *logrus.Entry
	*prober.EventEmitter
}

var _ manager.Component = (*OCIBundleReconciler)(nil)

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars *config.CfgVars) *OCIBundleReconciler {
	return &OCIBundleReconciler{
		k0sVars:      vars,
		log:          logrus.WithField("component", "OCIBundleReconciler"),
		EventEmitter: prober.NewEventEmitter(),
	}
}

func (a *OCIBundleReconciler) Init(_ context.Context) error {
	return dir.Init(a.k0sVars.OCIBundleDir, constant.ManifestsDirMode)
}

func (a *OCIBundleReconciler) Start(ctx context.Context) error {
	files, err := os.ReadDir(a.k0sVars.OCIBundleDir)
	if err != nil {
		a.Emit("can't read bundles directory")
		return fmt.Errorf("can't read bundles directory")
	}
	a.EmitWithPayload("importing OCI bundles", files)
	if len(files) == 0 {
		return nil
	}
	var client *containerd.Client
	sock := filepath.Join(a.k0sVars.RunDir, "containerd.sock")
	err = retry.Do(func() error {
		client, err = containerd.New(sock, containerd.WithDefaultNamespace("k8s.io"), containerd.WithDefaultPlatform(platforms.OnlyStrict(platforms.DefaultSpec())))
		if err != nil {
			a.log.WithError(err).Errorf("can't connect to containerd socket %s", sock)
			return err
		}
		_, err := client.ListImages(ctx)
		if err != nil {
			a.log.WithError(err).Errorf("can't use containerd client")
			return err
		}
		return nil
	}, retry.Context(ctx), retry.Delay(time.Second*5))
	if err != nil {
		a.EmitWithPayload("can't connect to containerd socket", map[string]interface{}{"socket": sock, "error": err})
		return fmt.Errorf("can't connect to containerd socket %s: %v", sock, err)
	}
	defer client.Close()

	for _, file := range files {
		if err := a.unpackBundle(ctx, client, a.k0sVars.OCIBundleDir+"/"+file.Name()); err != nil {
			a.EmitWithPayload("unpacking OCI bundle error", map[string]interface{}{"file": file.Name(), "error": err})
			a.log.WithError(err).Errorf("can't unpack bundle %s", file.Name())
			return fmt.Errorf("can't unpack bundle %s: %w", file.Name(), err)
		}
		a.EmitWithPayload("unpacked OCI bundle", file.Name())
	}
	a.Emit("finished importing OCI bundle")
	return nil
}

func (a OCIBundleReconciler) unpackBundle(ctx context.Context, client *containerd.Client, bundlePath string) error {
	r, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("can't open bundle file %s: %v", bundlePath, err)
	}
	defer r.Close()
	images, err := client.Import(ctx, r)
	if err != nil {
		return fmt.Errorf("can't import bundle: %v", err)
	}
	for _, i := range images {
		a.log.Infof("Imported image %s", i.Name)
	}
	return nil
}

func (a *OCIBundleReconciler) Stop() error {
	return nil
}
