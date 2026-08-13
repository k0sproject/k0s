// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/platforms"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	workercontainerd "github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/debounce"
)

const (
	// Follows a list of labels we use to control imported images.
	ImagePinnedLabel      = "io.cri-containerd.pinned"
	ImageSourcePathsLabel = "io.k0sproject.ocibundle-paths"

	// embeddedBundleDir is the directory inside the k0s executable's ZIP payload
	// that holds the embedded OCI bundles.
	embeddedBundleDir = "images"

	// embeddedSourcePrefix marks image source paths that refer to an OCI bundle
	// embedded in the k0s executable's ZIP payload, rather than to a file in the
	// OCI bundle directory. The remainder of such a path is the payload entry
	// name, e.g. "images/bundle.tar".
	//
	// Note that the k0s executable's path is deliberately not part of this, so
	// that images stay pinned when k0s is moved or reinstalled.
	embeddedSourcePrefix = "k0s-embedded://"
)

// OCIBundleReconciler tries to import OCI bundle into the running containerd instance
type OCIBundleReconciler struct {
	ociBundleDir      string
	containerdAddress string
	log               *logrus.Entry
	// openPayload opens the ZIP payload appended to the k0s executable.
	openPayload func() (*assets.Payload, error)
	// alreadyImported maps the names of the bundles that have been imported by
	// this reconciler to the version of their contents at the time.
	alreadyImported map[string]string
	mtx             sync.Mutex
	cancel          context.CancelFunc
	end             chan struct{}
	*prober.EventEmitter
}

var _ manager.Component = (*OCIBundleReconciler)(nil)

// NewOCIBundleReconciler builds new reconciler
func NewOCIBundleReconciler(vars *config.CfgVars) *OCIBundleReconciler {
	return &OCIBundleReconciler{
		ociBundleDir:      vars.OCIBundleDir,
		containerdAddress: workercontainerd.Address(vars.RunDir),
		log:               logrus.WithField("component", "OCIBundleReconciler"),
		openPayload:       assets.OpenSelfPayload,
		EventEmitter:      prober.NewEventEmitter(),
		alreadyImported:   map[string]string{},
		end:               make(chan struct{}),
	}
}

// bundleSource is an OCI image bundle that can be imported into containerd.
type bundleSource struct {
	// name identifies this bundle in the import bookkeeping and in the image
	// source label. Bundles in the OCI bundle directory use their path, embedded
	// bundles their payload entry name, prefixed with embeddedSourcePrefix.
	name string

	// version changes whenever the bundle's contents change. Only used to decide
	// if a bundle needs to be imported again.
	version string

	// modTime is the modification time recorded in the image source label.
	modTime time.Time

	// open returns a reader for the bundle's uncompressed tar stream. It is only
	// valid as long as the store backing this bundle is, i.e. as long as the
	// payload that an embedded bundle belongs to remains open.
	open func() (io.ReadCloser, error)
}

// bundleSourcesFromDir returns the OCI bundles in dir, which is expected to be
// the OCI bundle directory.
func bundleSourcesFromDir(dir string) ([]bundleSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	sources := make([]bundleSource, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			return sources, fmt.Errorf("failed to stat %s: %w", path, err)
		}
		if info.IsDir() {
			continue
		}

		modTime := info.ModTime()
		sources = append(sources, bundleSource{
			name: path,
			// Bundles in the OCI bundle directory are expected to be replaced
			// rather than modified in place, so their modification time is a
			// good enough indication of their contents.
			version: modTime.String(),
			modTime: modTime,
			open:    func() (io.ReadCloser, error) { return os.Open(path) },
		})
	}

	return sources, nil
}

// bundleSourcesFromPayload returns the OCI bundles that are embedded in
// payloadFS, the ZIP payload of the k0s executable. The modTime is the one
// recorded for those bundles in the image source label. The returned sources are
// only valid as long as the payload remains open.
func bundleSourcesFromPayload(payloadFS fs.FS, modTime time.Time) ([]bundleSource, error) {
	entries, err := fs.ReadDir(payloadFS, embeddedBundleDir)
	if err != nil {
		// Executables without any embedded bundles have no such directory. This
		// is the case for all the k0s executables that are shipped by the k0s
		// project, so it's not an error.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	sources := make([]bundleSource, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// ZIP entry names are always slash-separated, on all platforms.
		entryName := embeddedBundleDir + "/" + entry.Name()
		info, err := entry.Info()
		if err != nil {
			return sources, fmt.Errorf("failed to inspect %s: %w", entryName, err)
		}

		version, ok := assets.ContentID(info)
		if !ok {
			// Without a way to tell if the contents changed, the bundle has to
			// be imported on every start. Importing is idempotent, so the only
			// cost is the time it takes.
			version = ""
		}

		sources = append(sources, bundleSource{
			name:    embeddedSourcePrefix + entryName,
			version: version,
			// Embedded bundles are identified by their contents. Their recorded
			// modification time is irrelevant, but has to be stored, since it's
			// part of the image source label's format.
			modTime: modTime,
			open: func() (io.ReadCloser, error) {
				return payloadFS.Open(entryName)
			},
		})
	}

	return sources, nil
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
func (a *OCIBundleReconciler) loadOne(ctx context.Context, payloadFS fs.FS, src bundleSource) error {
	client, err := a.containerdClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer client.Close()
	if err := a.unpackBundle(ctx, client, payloadFS, src); err != nil {
		return fmt.Errorf("failed to process OCI bundle: %w", err)
	}
	return nil
}

// loadAll loads all the OCI bundles that are embedded in the k0s executable, as
// well as all the ones in the OCI bundle directory, into containerd. Errors are
// logged but not returned, upon failure in one bundle this function logs the
// error and moves to the next one. Bundles are indexed by name and imported only
// once, as long as their contents don't change.
func (a *OCIBundleReconciler) loadAll(ctx context.Context) {
	// We are going to consume every bundle, so we block. This keeps things simple
	// and avoids the need to handle two imports of the same bundle at the same
	// time without requiring locks based on bundle names.
	a.mtx.Lock()
	defer a.mtx.Unlock()

	a.log.Info("Loading OCI bundles")

	var sources []bundleSource

	// The payload has to stay open for as long as its bundles may be read, i.e.
	// for the whole duration of this call, including the unpinning below, which
	// needs to inspect the same set of embedded bundles.
	var payloadFS fs.FS
	payload, err := a.openPayload()
	switch {
	case err == nil:
		defer func() {
			if err := payload.Close(); err != nil {
				a.log.WithError(err).Warn("Failed to close payload")
			}
		}()
		payloadFS = payload

		embeddedSources, err := bundleSourcesFromPayload(payload, payload.ModTime())
		if err != nil {
			a.log.WithError(err).Error("Failed to collect the embedded OCI bundles")
		}
		sources = append(sources, embeddedSources...)

	case errors.Is(err, assets.ErrNoPayload):
		// A bare k0s executable has no embedded bundles. Images that claim to
		// originate from one are stale and may be unpinned.
		payloadFS = emptyFS{}

	default:
		// It's unknown if there are any embedded bundles. Leave payloadFS unset,
		// so that the unpinning below is skipped, instead of unpinning images
		// whose embedded bundles may well still be there.
		a.log.WithError(err).Error("Failed to open the k0s executable's payload")
	}

	// Collect the bundles from the OCI bundle directory after the embedded ones,
	// so that a bundle that's placed there takes precedence over an embedded one.
	dirSources, err := bundleSourcesFromDir(a.ociBundleDir)
	if err != nil {
		a.log.WithError(err).Error("Failed to collect the OCI bundles in the bundle directory")
	}
	sources = append(sources, dirSources...)

	names := make([]string, len(sources))
	for i, src := range sources {
		names[i] = src.name
	}
	a.EmitWithPayload("importing OCI bundles", names)

	for _, src := range sources {
		// Bundles whose contents cannot be identified have an empty version and
		// are imported again on every run.
		if version, ok := a.alreadyImported[src.name]; ok && src.version != "" && version == src.version {
			continue
		}

		log := a.log.WithField("bundle", src.name)
		log.Info("Loading OCI bundle")
		if err := a.loadOne(ctx, payloadFS, src); err != nil {
			log.WithError(err).Error("Failed to load OCI bundle")
			continue
		}

		a.alreadyImported[src.name] = src.version
		log.Info("OCI bundle loaded")
	}

	if payloadFS == nil {
		a.log.Warn("Not unpinning images: it's unknown which OCI bundles are embedded")
		a.Emit("skipped unpinning images")
	} else if err := a.unpinAll(ctx, payloadFS); err != nil {
		a.log.WithError(err).Errorf("Failed to unpin images")
	}

	a.Emit("finished importing OCI bundles")
}

// emptyFS is an fs.FS without any files in it.
type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// unpin unpins containerd images from the image store. we unpin an image if
// the bundle from where it was imported no longer exists or its content has
// been changed. Bundles are looked up in the local file system, or, for embedded
// bundles, in payloadFS, the ZIP payload of the k0s executable.
func (a *OCIBundleReconciler) unpinAll(ctx context.Context, payloadFS fs.FS) error {
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
		if err := a.unpinOne(ctx, image, isvc, payloadFS); err != nil {
			a.log.WithError(err).Errorf("Failed to unpin image %s", image.Name)
		}
	}
	return nil
}

// unpinOne checks if we can unpin the provided image and if so unpins it.
func (a *OCIBundleReconciler) unpinOne(ctx context.Context, image images.Image, isvc images.Store, payloadFS fs.FS) error {
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
	if exists, err := sources.Exist(payloadFS); err != nil {
		return fmt.Errorf("failed to check if sources exist: %w", err)
	} else if exists {
		if err := sources.Refresh(payloadFS); err != nil {
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

// installWatcher creates a fs watcher on the oci bundle directory. This function calls
// loadAll every time a new file is created or updated on the oci directory. Events are
// debounced with a timeout of 10 seconds. Watcher is started with a buffer so we don't
// miss events.
func (a *OCIBundleReconciler) installWatcher(ctx context.Context) error {
	watcher, err := fsnotify.NewBufferedWatcher(10)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(a.ociBundleDir); err != nil {
		return fmt.Errorf("failed to add watcher: %w", err)
	}

	debouncer := debounce.Debouncer[fsnotify.Event]{
		Input:   watcher.Events,
		Timeout: 10 * time.Second,
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
		a.log.Infof("Started to watch events on %s", a.ociBundleDir)
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

// unpackBundle imports the bundle into the containerd storage. imported images are
// pinned and labeled so we can control them later.
func (a *OCIBundleReconciler) unpackBundle(ctx context.Context, client *containerd.Client, payloadFS fs.FS, src bundleSource) error {
	// Embedded bundles are streamed straight out of the k0s executable's ZIP
	// payload, i.e. they are never extracted to disk.
	r, err := src.open()
	if err != nil {
		return fmt.Errorf("can't open bundle %s: %w", src.name, err)
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
		if err := AddToImageSources(&i, payloadFS, src.name, src.modTime); err != nil {
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
	a.cancel()
	<-a.end
	a.log.Info("OCI bundle loader stopped")
	return nil
}

// ImageSources holds a map of bundle paths with their respective modification times.
// this is used to track from which bundles a given image was imported.
//
// Paths prefixed with embeddedSourcePrefix refer to bundles embedded in the k0s
// executable's ZIP payload, all the other paths to files in the node's file
// system.
type ImageSources map[string]time.Time

// exists checks if the bundle identified by path is still available, and still
// has the given modification time. Embedded bundles are looked up in payloadFS,
// the ZIP payload of the k0s executable.
func bundleSourceExists(payloadFS fs.FS, path string, modtime time.Time) (bool, error) {
	if name, embedded := strings.CutPrefix(path, embeddedSourcePrefix); embedded {
		// Embedded bundles are identified by their contents, which are already
		// known to be unchanged if the payload still contains them. Comparing
		// modification times here would unpin images whenever the k0s
		// executable's modification time changes, which happens for reasons that
		// have nothing to do with its contents, such as being reinstalled.
		// Unusable names are reported as an error, not as non-existing. The
		// latter would unpin the image, whereas an error leaves it pinned until
		// this is sorted out. Check this explicitly, since file systems differ in
		// how they report invalid names.
		if !fs.ValidPath(name) {
			return false, fmt.Errorf("%w: invalid embedded bundle name %q", fs.ErrInvalid, name)
		}
		if _, err := fs.Stat(payloadFS, name); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("failed to stat embedded bundle %s: %w", name, err)
		}
		return true, nil
	}

	finfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat %s: %w", path, err)
	}
	return finfo.ModTime().Equal(modtime), nil
}

// Refresh removes from the list of source paths all the paths that no longer exists
// or have been modified.
func (i *ImageSources) Refresh(payloadFS fs.FS) error {
	newmap := map[string]time.Time{}
	for path, modtime := range *i {
		exists, err := bundleSourceExists(payloadFS, path, modtime)
		if err != nil {
			return err
		}
		if exists {
			newmap[path] = modtime
		}
	}
	*i = newmap
	return nil
}

// Exist returns true if a given bundle source is still available, either in the
// node's file system or in the k0s executable's ZIP payload.
func (i *ImageSources) Exist(payloadFS fs.FS) (bool, error) {
	for path, modtime := range *i {
		exists, err := bundleSourceExists(payloadFS, path, modtime)
		if err != nil {
			return false, err
		}
		if exists {
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
func AddToImageSources(image *images.Image, payloadFS fs.FS, path string, modtime time.Time) error {
	paths, err := GetImageSources(*image)
	if err != nil {
		return fmt.Errorf("failed to get image sources: %w", err)
	}
	if err := paths.Refresh(payloadFS); err != nil {
		return fmt.Errorf("failed to refresh image sources: %w", err)
	}
	paths[path] = modtime
	return SetImageSources(image, paths)
}
