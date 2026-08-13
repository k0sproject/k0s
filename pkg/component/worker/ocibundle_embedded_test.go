// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"archive/zip"
	"context"
	"encoding/json"
	"hash/crc32"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0sproject/k0s/pkg/component/prober"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// payloadEntry builds an fstest.MapFile that carries the same ZIP metadata that
// assets.ContentID relies on, so that embedded bundles can be identified by
// their contents in tests.
func payloadEntry(contents string) *fstest.MapFile {
	data := []byte(contents)
	return &fstest.MapFile{
		Data: data,
		Sys: &zip.FileHeader{
			CRC32:              crc32.ChecksumIEEE(data),
			UncompressedSize64: uint64(len(data)),
		},
	}
}

func TestBundleSourcesFromPayload(t *testing.T) {
	modTime := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

	// The k0s executables shipped by the k0s project have no embedded bundles at
	// all. That's not an error, there's simply nothing to import.
	t.Run("noEmbeddedBundles", func(t *testing.T) {
		sources, err := bundleSourcesFromPayload(fstest.MapFS{
			"bin/containerd": payloadEntry("containerd"),
		}, modTime)
		assert.NoError(t, err)
		assert.Empty(t, sources)
	})

	t.Run("collectsBundles", func(t *testing.T) {
		sources, err := bundleSourcesFromPayload(fstest.MapFS{
			"bin/containerd":    payloadEntry("containerd"),
			"images/first.tar":  payloadEntry("first bundle"),
			"images/second.tar": payloadEntry("second bundle"),
			// Bundles are collected non-recursively, mirroring the behavior for
			// the OCI bundle directory.
			"images/nested/third.tar": payloadEntry("third bundle"),
		}, modTime)
		require.NoError(t, err)

		names := make([]string, len(sources))
		for i, src := range sources {
			names[i] = src.name
			assert.True(t, modTime.Equal(src.modTime), "Unexpected modification time for %s", src.name)
			assert.NotEmpty(t, src.version, "Expected %s to be identifiable by its contents", src.name)
		}
		assert.Equal(t, []string{
			"k0s-embedded://images/first.tar",
			"k0s-embedded://images/second.tar",
		}, names)
	})

	// Bundles are streamed out of the payload, they are never extracted to disk.
	t.Run("opensBundleContents", func(t *testing.T) {
		sources, err := bundleSourcesFromPayload(fstest.MapFS{
			"images/some.tar": payloadEntry("some bundle"),
		}, modTime)
		require.NoError(t, err)
		require.Len(t, sources, 1)

		reader, err := sources[0].open()
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, reader.Close()) })
		contents, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "some bundle", string(contents))
	})

	// Bundles whose contents can't be identified have to be imported again on
	// every run, which is indicated by an empty version.
	t.Run("unidentifiableContents", func(t *testing.T) {
		sources, err := bundleSourcesFromPayload(fstest.MapFS{
			"images/some.tar": {Data: []byte("some bundle")},
		}, modTime)
		require.NoError(t, err)
		require.Len(t, sources, 1)
		assert.Empty(t, sources[0].version)
	})
}

func TestBundleSourcesFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "some.tar"), []byte("some bundle"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0755))

	sources, err := bundleSourcesFromDir(dir)
	require.NoError(t, err)
	require.Len(t, sources, 1, "Directories should be skipped")

	assert.Equal(t, filepath.Join(dir, "some.tar"), sources[0].name)
	assert.NotEmpty(t, sources[0].version)
	reader, err := sources[0].open()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, reader.Close()) })
	contents, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "some bundle", string(contents))
}

func TestImageSources_EmbeddedBundles(t *testing.T) {
	payloadFS := fstest.MapFS{"images/some.tar": payloadEntry("some bundle")}

	// Embedded bundles are identified by their contents, so the recorded
	// modification time must not be taken into account. Otherwise images would
	// get unpinned whenever the k0s executable's modification time changes, e.g.
	// when it gets reinstalled.
	t.Run("ignoresRecordedModTime", func(t *testing.T) {
		for _, modTime := range []time.Time{{}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)} {
			sources := ImageSources{"k0s-embedded://images/some.tar": modTime}
			exists, err := sources.Exist(payloadFS)
			require.NoError(t, err)
			assert.True(t, exists, "Expected the embedded bundle to exist for modification time %s", modTime)

			require.NoError(t, sources.Refresh(payloadFS))
			assert.Len(t, sources, 1, "Expected the embedded bundle to be retained")
		}
	})

	t.Run("goneFromPayload", func(t *testing.T) {
		sources := ImageSources{"k0s-embedded://images/other.tar": time.Now()}
		exists, err := sources.Exist(payloadFS)
		require.NoError(t, err)
		assert.False(t, exists)

		require.NoError(t, sources.Refresh(payloadFS))
		assert.Empty(t, sources)
	})

	// Bare k0s executables have no payload at all, so any embedded bundle that an
	// image refers to is gone.
	t.Run("noPayload", func(t *testing.T) {
		sources := ImageSources{"k0s-embedded://images/some.tar": time.Now()}
		exists, err := sources.Exist(emptyFS{})
		require.NoError(t, err)
		assert.False(t, exists)
	})

	// Embedded bundle names are not paths in the node's file system. They must
	// never be resolved as such, no matter how they look. Names that aren't valid
	// payload entry names are reported as an error rather than as non-existing,
	// since the latter would unpin the image.
	t.Run("notResolvedInFileSystem", func(t *testing.T) {
		dir := t.TempDir()
		decoy := filepath.Join(dir, "decoy.tar")
		require.NoError(t, os.WriteFile(decoy, []byte("decoy"), 0644))
		info, err := os.Stat(decoy)
		require.NoError(t, err)

		sources := ImageSources{embeddedSourcePrefix + decoy: info.ModTime()}
		exists, err := sources.Exist(payloadFS)
		assert.ErrorIs(t, err, fs.ErrInvalid)
		assert.False(t, exists, "An embedded bundle must not be resolved in the file system")
	})

	// Sources of both kinds may be recorded for the same image.
	t.Run("mixedWithFileSystemSources", func(t *testing.T) {
		dir := t.TempDir()
		onDisk := filepath.Join(dir, "on-disk.tar")
		require.NoError(t, os.WriteFile(onDisk, []byte("on disk"), 0644))
		info, err := os.Stat(onDisk)
		require.NoError(t, err)

		sources := ImageSources{
			onDisk:                            info.ModTime(),
			"k0s-embedded://images/some.tar":  time.Time{},
			"k0s-embedded://images/other.tar": time.Time{},
		}

		require.NoError(t, sources.Refresh(payloadFS))
		assert.Equal(t, []string{onDisk, "k0s-embedded://images/some.tar"}, sortedKeys(sources),
			"Expected only the vanished embedded bundle to be dropped")

		// Removing the file on disk leaves the embedded bundle as the only source.
		require.NoError(t, os.Remove(onDisk))
		require.NoError(t, sources.Refresh(payloadFS))
		assert.Equal(t, []string{"k0s-embedded://images/some.tar"}, sortedKeys(sources))

		exists, err := sources.Exist(payloadFS)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

// fakeImageStore is an in-memory images.Store that records the updates made to
// it.
type fakeImageStore struct {
	images.Store
	updates []images.Image
	fields  [][]string
}

func (s *fakeImageStore) Update(_ context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	s.updates = append(s.updates, image)
	s.fields = append(s.fields, fieldpaths)
	return image, nil
}

func TestUnpinOne(t *testing.T) {
	payloadFS := fstest.MapFS{"images/some.tar": payloadEntry("some bundle")}

	pinnedImage := func(t *testing.T, sources ImageSources) images.Image {
		image := images.Image{
			Name:   "example.com/some/image:latest",
			Labels: map[string]string{ImagePinnedLabel: "pinned"},
		}
		if len(sources) > 0 {
			data, err := json.Marshal(sources)
			require.NoError(t, err)
			image.Labels[ImageSourcePathsLabel] = string(data)
		}
		return image
	}

	unpinOne := func(t *testing.T, image images.Image, payloadFS fs.FS) *fakeImageStore {
		reconciler := &OCIBundleReconciler{
			log:          logrus.WithField("test", t.Name()),
			EventEmitter: prober.NewEventEmitter(),
		}
		store := new(fakeImageStore)
		require.NoError(t, reconciler.unpinOne(context.TODO(), image, store, payloadFS))
		return store
	}

	// As long as the embedded bundle is still there, the image stays pinned.
	// Unpinning it would allow the kubelet to garbage collect an image that
	// cannot be pulled again on an air-gapped node.
	t.Run("keepsPinWhileEmbeddedBundleExists", func(t *testing.T) {
		image := pinnedImage(t, ImageSources{"k0s-embedded://images/some.tar": time.Time{}})
		store := unpinOne(t, image, payloadFS)

		require.Len(t, store.updates, 1)
		assert.Equal(t, "pinned", store.updates[0].Labels[ImagePinnedLabel])
		assert.Equal(t, [][]string{{"labels." + ImageSourcePathsLabel}}, store.fields,
			"Only the image sources should have been updated")
	})

	t.Run("unpinsWhenEmbeddedBundleIsGone", func(t *testing.T) {
		image := pinnedImage(t, ImageSources{"k0s-embedded://images/other.tar": time.Time{}})
		store := unpinOne(t, image, payloadFS)

		require.Len(t, store.updates, 1)
		assert.NotContains(t, store.updates[0].Labels, ImagePinnedLabel)
		assert.NotContains(t, store.updates[0].Labels, ImageSourcePathsLabel)
	})

	// Images that k0s didn't import have no image sources recorded. They may be
	// pinned by someone else, e.g. containerd pins its own sandbox image.
	t.Run("ignoresForeignImages", func(t *testing.T) {
		store := unpinOne(t, pinnedImage(t, nil), payloadFS)
		assert.Empty(t, store.updates, "Foreign images should be left alone")
	})

	t.Run("ignoresUnpinnedImages", func(t *testing.T) {
		image := pinnedImage(t, ImageSources{"k0s-embedded://images/other.tar": time.Time{}})
		delete(image.Labels, ImagePinnedLabel)
		store := unpinOne(t, image, payloadFS)
		assert.Empty(t, store.updates)
	})
}

func TestEmptyFS(t *testing.T) {
	_, err := fs.Stat(emptyFS{}, "images/some.tar")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	_, err = fs.ReadDir(emptyFS{}, embeddedBundleDir)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	_, err = emptyFS{}.Open("../escaping")
	assert.ErrorIs(t, err, fs.ErrInvalid)
}

// sortedKeys returns the keys of sources in ascending order.
func sortedKeys(sources ImageSources) []string {
	return slices.Sorted(maps.Keys(sources))
}
