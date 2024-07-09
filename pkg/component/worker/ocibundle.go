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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/debounce"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/avast/retry-go"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	k0sVars         *config.CfgVars
	log             *logrus.Entry
	alreadyImported map[string]time.Time
	mtx             sync.Mutex
	cancel          context.CancelFunc
	end             chan struct{}
	*prober.EventEmitter
}

var _ manager.Component = (*OCIBundleReconciler)(nil)

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars *config.CfgVars) *OCIBundleReconciler {
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
			_ = client.Close()
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
	images, err := importBundle(ctx, client, r)
	if err != nil {
		return fmt.Errorf("can't import bundle: %v", err)
	}
	is := client.ImageService()
	for _, i := range images {
		a.log.Infof("Imported image %s", i.Name)
		// Update labels for each image to include io.cri-containerd.pinned=pinned
		fieldpaths := []string{"labels.io.cri-containerd.pinned"}
		if i.Labels == nil {
			i.Labels = make(map[string]string)
		}
		i.Labels["io.cri-containerd.pinned"] = "pinned"
		_, err := is.Update(ctx, i, fieldpaths...)
		if err != nil {
			return fmt.Errorf("failed to add io.cri-containerd.pinned label for image %s: %w", i.Name, err)
		}
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

// SPDX-SnippetBegin
// SPDX-SnippetCopyrightText: The containerd Authors.
// SPDX-SnippetCopyrightText: 2024 k0s authors
// SDPX—SnippetName: Adapted version of containerd.Client.Import
// SPDX-SnippetComment: Includes changes from https://github.com/containerd/containerd/pull/9554/commits/61a7c4999c78e70f0be672c587feed501f9144f2#diff-ba1db69d961491f72eaba3134ca05b5d8c93626791299eadee38bb9f6cd71db3R175-R180

// importBundle imports an image from a Tar stream using reader.
// Caller needs to specify importer. Future version may use oci.v1 as the default.
// Note that unreferenced blobs may be imported to the content store as well.
func importBundle(ctx context.Context, c *containerd.Client, reader io.Reader) (_ []images.Image, err error) {
	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, done(ctx)) }()

	index, err := archive.ImportIndex(ctx, c.ContentStore(), reader)
	if err != nil {
		return nil, err
	}

	var (
		imgs []images.Image
		cs   = c.ContentStore()
		is   = c.ImageService()
	)

	var platformMatcher = platforms.Only(platforms.DefaultSpec())

	var handler images.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		// Only save images at top level
		if desc.Digest != index.Digest {
			// Don't set labels on missing content.
			children, err := images.Children(ctx, cs, desc)

			// Without this the importing would fail if the bundle does not images for compatible architectures
			// because the image manifest still refers to those. E.g. on arm64 containerd would stil try to unpack arm/v8&arm/v7
			// images but would fail as those are not present on k0s airgap bundles.
			if errdefs.IsNotFound(err) {
				return nil, images.ErrSkipDesc
			}
			return children, err
		}

		p, err := content.ReadBlob(ctx, cs, desc)
		if err != nil {
			return nil, err
		}

		var idx ocispec.Index
		if err := json.Unmarshal(p, &idx); err != nil {
			return nil, err
		}

		for _, m := range idx.Manifests {
			name := m.Annotations[images.AnnotationImageName]
			if name == "" {
				name = m.Annotations[ocispec.AnnotationRefName]
			}
			if name != "" {
				imgs = append(imgs, images.Image{
					Name:   name,
					Target: m,
				})
			}
		}

		return idx.Manifests, nil
	}

	handler = images.FilterPlatforms(handler, platformMatcher)
	handler = images.SetChildrenLabels(cs, handler)
	if err := images.WalkNotEmpty(ctx, handler, index); err != nil {
		return nil, err
	}

	for i := range imgs {
		img, err := is.Update(ctx, imgs[i], "target")
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, err
			}

			img, err = is.Create(ctx, imgs[i])
			if err != nil {
				return nil, err
			}
		}
		imgs[i] = img
	}

	return imgs, nil
}

// SPDX-SnippetEnd
