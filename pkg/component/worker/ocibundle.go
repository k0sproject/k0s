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
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/platforms"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/debounce"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	k0sVars         constant.CfgVars
	log             *logrus.Entry
	alreadyImported map[string]time.Time
	mtx             sync.Mutex
	cancel          context.CancelFunc
	end             chan struct{}
	*prober.EventEmitter
}

var _ manager.Component = (*OCIBundleReconciler)(nil)

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars constant.CfgVars) *OCIBundleReconciler {
	return &OCIBundleReconciler{
		k0sVars:         vars,
		log:             logrus.WithField("component", "OCIBundleReconciler"),
		EventEmitter:    prober.NewEventEmitter(),
		alreadyImported: map[string]time.Time{},
		end:             make(chan struct{}),
	}
}

func (a *OCIBundleReconciler) Init(_ context.Context) error {
	return dir.Init(a.k0sVars.OCIBundleDir, constant.ManifestsDirMode)
}

// loadOne connects to containerd and imports the provided OCI bundle.
func (a *OCIBundleReconciler) loadOne(ctx context.Context, fpath string) error {
	var client *containerd.Client
	sock := filepath.Join(a.k0sVars.RunDir, "containerd.sock")
	if err := retry.Do(func() (err error) {
		client, err = containerd.New(
			sock,
			containerd.WithDefaultNamespace("k8s.io"),
			containerd.WithDefaultPlatform(
				platforms.Only(platforms.DefaultSpec()),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to containerd: %w", err)
		}
		if _, err = client.ListImages(ctx); err != nil {
			return fmt.Errorf("failed to communicate with containerd: %w", err)
		}
		return nil
	}, retry.Context(ctx), retry.Delay(time.Second*5)); err != nil {
		return err
	}
	defer client.Close()
	if err := a.unpackBundle(ctx, client, fpath); err != nil {
		return fmt.Errorf("failed to process OCI bundle: %w", err)
	}
	return nil
}

// loadAll loads all OCI bundle files into containerd. Read all files from the OCI bundle
// directory and loads them one by one. Errors are logged but not returned, upon failure
// in one file this function logs the error and moves to the next file. Files are indexed
// by name and imported only once (if the file has not been modified).
func (a *OCIBundleReconciler) loadAll(ctx context.Context) {
	// We are going to consume everything in the directory so we block. This keeps
	// things simple and avoid the need to handle two imports of the same file at the
	// same time without requiring locks based on file path.
	a.mtx.Lock()
	defer a.mtx.Unlock()

	a.log.Info("Loading OCI bundles directory")
	files, err := os.ReadDir(a.k0sVars.OCIBundleDir)
	if err != nil {
		a.log.WithError(err).Errorf("Failed to read bundles directory")
		return
	}
	a.EmitWithPayload("importing OCI bundles", files)
	for _, file := range files {
		fpath := filepath.Join(a.k0sVars.OCIBundleDir, file.Name())
		finfo, err := os.Stat(fpath)
		if err != nil {
			a.log.WithError(err).Errorf("failed to stat %s", fpath)
			continue
		}

		modtime := finfo.ModTime()
		if when, ok := a.alreadyImported[fpath]; ok && when.Equal(modtime) {
			continue
		}

		a.log.Infof("Loading OCI bundle %s", fpath)
		if err := a.loadOne(ctx, fpath); err != nil {
			a.log.WithError(err).Errorf("Failed to load OCI bundle %s", fpath)
			continue
		}

		a.alreadyImported[fpath] = modtime
		a.log.Infof("OCI bundle %s loaded", fpath)
	}
	a.Emit("finished importing OCI bundles")
}

// installWatcher creates a fs watcher on the oci bundle directory. This function calls
// loadAll every time a new file is created or updated on the oci directory. Events are
// debounced with a timeout of 10 seconds. Watcher is started with a buffer so we don't
// miss events.
func (a *OCIBundleReconciler) installWatcher(ctx context.Context) error {
	watcher, err := fsnotify.NewBufferedWatcher(10)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(a.k0sVars.OCIBundleDir); err != nil {
		return fmt.Errorf("failed to add watcher: %w", err)
	}

	debouncer := debounce.Debouncer[fsnotify.Event]{
		Input:   watcher.Events,
		Timeout: 10 * time.Second,
		Filter: func(item fsnotify.Event) bool {
			switch item.Op {
			case fsnotify.Remove, fsnotify.Rename:
				return false
			}
			return true
		},
		Callback: func(ev fsnotify.Event) {
			a.loadAll(ctx)
		},
	}

	go func() {
		for {
			if err, ok := <-watcher.Errors; ok {
				a.log.WithError(err).Error("Error watching OCI bundle directory")
				continue
			}
			return
		}
	}()

	go func() {
		defer close(a.end)
		a.log.Infof("Started to watch events on %s", a.k0sVars.OCIBundleDir)
		_ = debouncer.Run(ctx)
		if err := watcher.Close(); err != nil {
			a.log.Errorf("Failed to close watcher: %s", err)
		}
		a.log.Info("OCI bundle watch bouncer ended")
	}()

	return nil
}

// Starts initiate the OCI bundle loader. It does an initial load of the directory and
// once it is done, it starts a watcher on its own goroutine.
func (a *OCIBundleReconciler) Start(ctx context.Context) error {
	ictx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	if err := a.installWatcher(ictx); err != nil {
		return fmt.Errorf("failed to install watcher: %w", err)
	}
	a.loadAll(ictx)
	return nil
}

func (a *OCIBundleReconciler) unpackBundle(ctx context.Context, client *containerd.Client, bundlePath string) error {
	r, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("can't open bundle file %s: %v", bundlePath, err)
	}
	defer r.Close()
	// WithSkipMissing allows us to skip missing blobs
	// Without this the importing would fail if the bundle does not images for compatible architectures
	// because the image manifest still refers to those. E.g. on arm64 containerd would stil try to unpack arm/v8&arm/v7
	// images but would fail as those are not present on k0s airgap bundles.
	images, err := client.Import(ctx, r, containerd.WithSkipMissing())
	if err != nil {
		return fmt.Errorf("can't import bundle: %v", err)
	}
	for _, i := range images {
		a.log.Infof("Imported image %s", i.Name)
	}
	return nil
}

func (a *OCIBundleReconciler) Stop() error {
	a.log.Info("Stopping OCI bundle loader watcher")
	a.cancel()
	<-a.end
	a.log.Info("OCI bundle loader stopped")
	return nil
}
