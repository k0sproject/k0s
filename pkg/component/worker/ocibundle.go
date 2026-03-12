// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/platforms"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

	oswatch "github.com/k0sproject/k0s/internal/os/watch"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	workercontainerd "github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

const (
	// Follows a list of labels we use to control imported images.
	ImagePinnedLabel      = "io.cri-containerd.pinned"
	ImageSourcePathsLabel = "io.k0sproject.ocibundle-paths"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	ociBundleDir      string
	containerdAddress string
	log               *logrus.Entry
	alreadyImported   map[string]time.Time
	stop              func()
	*prober.EventEmitter
}

var _ manager.Component = (*OCIBundleReconciler)(nil)

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars *config.CfgVars) *OCIBundleReconciler {
	return &OCIBundleReconciler{
		ociBundleDir:      vars.OCIBundleDir,
		containerdAddress: workercontainerd.Address(vars.RunDir),
		log:               logrus.WithField("component", "OCIBundleReconciler"),
		EventEmitter:      prober.NewEventEmitter(),
		alreadyImported:   map[string]time.Time{},
	}
}

func (a *OCIBundleReconciler) Init(_ context.Context) error {
	return dir.Init(a.ociBundleDir, constant.ManifestsDirMode)
}

// containerdClient returns a connected containerd client.
func (a *OCIBundleReconciler) containerdClient(ctx context.Context) (*containerd.Client, error) {
	var client *containerd.Client
	if err := retry.Do(func() (err error) {
		client, err = containerd.New(
			a.containerdAddress,
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
		return nil, err
	}
	return client, nil
}

// loadOne connects to containerd and imports the provided OCI bundle.
func (a *OCIBundleReconciler) loadOne(ctx context.Context, fpath string, modtime time.Time) error {
	client, err := a.containerdClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer client.Close()
	if err := a.unpackBundle(ctx, client, fpath, modtime); err != nil {
		return fmt.Errorf("failed to process OCI bundle: %w", err)
	}
	return nil
}

// loadAll loads all OCI bundle files into containerd. Read all files from the OCI bundle
// directory and loads them one by one. Errors are logged but not returned, upon failure
// in one file this function logs the error and moves to the next file. Files are indexed
// by name and imported only once (if the file has not been modified).
func (a *OCIBundleReconciler) loadAll(ctx context.Context) {
	a.log.Info("Loading OCI bundles directory")
	files, err := os.ReadDir(a.ociBundleDir)
	if err != nil {
		a.log.WithError(err).Errorf("Failed to read bundles directory")
		return
	}
	a.EmitWithPayload("importing OCI bundles", files)
	for _, file := range files {
		fpath := filepath.Join(a.ociBundleDir, file.Name())
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
		if err := a.loadOne(ctx, fpath, modtime); err != nil {
			a.log.WithError(err).Errorf("Failed to load OCI bundle %s", fpath)
			continue
		}

		a.alreadyImported[fpath] = modtime
		a.log.Infof("OCI bundle %s loaded", fpath)
	}

	if err := a.unpinAll(ctx); err != nil {
		a.log.WithError(err).Errorf("Failed to unpin images")
	}

	a.Emit("finished importing OCI bundles")
}

// unpin unpins containerd images from the image store. we unpin an image if
// the file from where it was imported no longer exists or the file content
// has been changed.
func (a *OCIBundleReconciler) unpinAll(ctx context.Context) error {
	client, err := a.containerdClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer client.Close()

	isvc := client.ImageService()
	images, err := isvc.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	for _, image := range images {
		if err := a.unpinOne(ctx, image, isvc); err != nil {
			a.log.WithError(err).Errorf("Failed to unpin image %s", image.Name)
		}
	}
	return nil
}

// unpinOne checks if we can unpin the provided image and if so unpins it.
func (a *OCIBundleReconciler) unpinOne(ctx context.Context, image images.Image, isvc images.Store) error {
	// if this image isn't pinned, return immediately.
	if v, pin := image.Labels[ImagePinnedLabel]; !pin || v != "pinned" {
		return nil
	}

	// extract the bundle paths from the image labels. if none has been found
	// then we don't own this image. return.
	sources, err := GetImageSources(image)
	if err != nil {
		return fmt.Errorf("failed to extract image source: %w", err)
	} else if len(sources) == 0 {
		return nil
	}

	// if any of the registered sources is still present, we can't unpin the image.
	// we just update the image label to remove references to the bundles that no
	// longer exist.
	if exists, err := sources.Exist(); err != nil {
		return fmt.Errorf("failed to check if sources exist: %w", err)
	} else if exists {
		if err := sources.Refresh(); err != nil {
			return fmt.Errorf("failed to refresh image sources: %w", err)
		}
		if err := SetImageSources(&image, sources); err != nil {
			return fmt.Errorf("failed to reset image sources: %w", err)
		}
		_, err := isvc.Update(ctx, image, "labels."+ImageSourcePathsLabel)
		return err
	}

	// all bundles referred by this image are no more, we can unpin it.
	a.log.Infof("Unpinning image %s", image.Name)
	a.EmitWithPayload("unpinning image", image.Name)
	delete(image.Labels, ImagePinnedLabel)
	delete(image.Labels, ImageSourcePathsLabel)
	_, err = isvc.Update(ctx, image)
	return err
}

// Watches the OCI bundle directory. This function calls loadAll every time a
// new file is created or updated in the OCI directory. Events are debounced
// with a timeout of 10 seconds.
func (a *OCIBundleReconciler) runWatcher(ctx context.Context, started *value.Once[error]) {
	err := oswatch.OnDirChange{
		InitialDelay: 1 * time.Second,
		Delay:        10 * time.Second,
	}.Run(ctx, a.ociBundleDir, func(ctx context.Context) error {
		a.loadAll(ctx)
		started.Set(nil)
		return nil
	})

	select {
	case <-ctx.Done():
	default:
		if err := cmp.Or(err, errors.New("watch terminated unexpectedly")); !started.Set(err) {
			a.log.WithError(err).Error("Failed to watch OCI bundle directory")
		}
	}
}

// Starts initiate the OCI bundle loader. It does an initial load of the directory and
// once it is done, it starts a watcher on its own goroutine.
func (a *OCIBundleReconciler) Start(startCtx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan struct{})

	var started value.Once[error]

	go func() {
		defer close(done)
		wait.JitterUntilWithContext(ctx, func(ctx context.Context) {
			a.runWatcher(ctx, &started)
		}, 1*time.Minute, 0.3, true)
	}()

	defer func() {
		if err == nil {
			a.stop = func() {
				cancel(errors.New("OCI bundle reconciler is stopping"))
				<-done
			}
		} else {
			cancel(err)
			<-done
		}
	}()

	select {
	case <-started.Done():
		return started.Await()

	case <-startCtx.Done():
		return context.Cause(startCtx)
	}
}

// unpackBundle imports the bundle into the containerd storage. imported images are
// pinned and labeled so we can control them later.
func (a *OCIBundleReconciler) unpackBundle(ctx context.Context, client *containerd.Client, bundlePath string, modtime time.Time) error {
	r, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("can't open bundle file %s: %w", bundlePath, err)
	}
	defer r.Close()
	// WithSkipMissing allows us to skip missing blobs
	// Without this the importing would fail if the bundle does not images for compatible architectures
	// because the image manifest still refers to those. E.g. on arm64 containerd would still try to unpack arm/v8&arm/v7
	// images but would fail as those are not present on k0s airgap bundles.
	images, err := client.Import(ctx, r, containerd.WithSkipMissing())
	if err != nil {
		return fmt.Errorf("can't import bundle: %w", err)
	}

	fieldpaths := []string{
		"labels." + ImagePinnedLabel,
		"labels." + ImageSourcePathsLabel,
	}

	isvc := client.ImageService()
	for _, i := range images {
		// here we add a label to pin the image in the containerd storage and another
		// to indicate from which oci buncle (file path) the image was imported from.
		a.log.Infof("Imported image %s", i.Name)

		if i.Labels == nil {
			i.Labels = make(map[string]string)
		}

		i.Labels[ImagePinnedLabel] = "pinned"
		if err := AddToImageSources(&i, bundlePath, modtime); err != nil {
			return fmt.Errorf("failed to add image source: %w", err)
		}

		if _, err := isvc.Update(ctx, i, fieldpaths...); err != nil {
			return fmt.Errorf("failed to add labels for image %s: %w", i.Name, err)
		}
	}
	return nil
}

func (a *OCIBundleReconciler) Stop() error {
	a.log.Info("Stopping OCI bundle loader watcher")
	a.stop()
	a.log.Info("OCI bundle loader stopped")
	return nil
}

// ImageSources holds a map of bundle paths with their respective modification times.
// this is used to track from which bundles a given image was imported.
type ImageSources map[string]time.Time

// Refresh removes from the list of source paths all the paths that no longer exists
// or have been modified.
func (i *ImageSources) Refresh() error {
	newmap := map[string]time.Time{}
	for path, modtime := range *i {
		finfo, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}
		if finfo.ModTime().Equal(modtime) {
			newmap[path] = modtime
		}
	}
	*i = newmap
	return nil
}

// Exist returns true if a given bundle source file still exists in the node fs.
func (i *ImageSources) Exist() (bool, error) {
	for path, modtime := range *i {
		finfo, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("failed to stat %s: %w", path, err)
		}
		if finfo.ModTime().Equal(modtime) {
			return true, nil
		}
	}
	return false, nil
}

// GetImageSources parses the image source label and returns the ImageSources. if
// no label has been set in the image this returns an empty but initiated map.
func GetImageSources(image images.Image) (ImageSources, error) {
	paths := map[string]time.Time{}
	value, found := image.Labels[ImageSourcePathsLabel]
	if !found {
		return paths, nil
	}
	if err := json.Unmarshal([]byte(value), &paths); err != nil {
		return nil, fmt.Errorf("failed to unmarshal label: %w", err)
	}
	return paths, nil
}

// SetImageSources sets the image source label in the image. this function will
// trim out of the sources the ones that no longer exists in the node fs.
func SetImageSources(image *images.Image, sources ImageSources) error {
	if len(sources) == 0 {
		return nil
	}
	data, err := json.Marshal(sources)
	if err != nil {
		return fmt.Errorf("failed to marshal image source: %w", err)
	}
	if image.Labels == nil {
		image.Labels = map[string]string{}
	}
	image.Labels[ImageSourcePathsLabel] = string(data)
	return nil
}

// AddToImageSources adds a new source path to the image sources. this function
// will trim out of the sources the ones that no longer exists in the node fs.
func AddToImageSources(image *images.Image, path string, modtime time.Time) error {
	paths, err := GetImageSources(*image)
	if err != nil {
		return fmt.Errorf("failed to get image sources: %w", err)
	}
	if err := paths.Refresh(); err != nil {
		return fmt.Errorf("failed to refresh image sources: %w", err)
	}
	paths[path] = modtime
	return SetImageSources(image, paths)
}
