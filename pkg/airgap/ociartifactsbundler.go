// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"archive/tar"
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"os"
	"path"
	"slices"
	"sync"

	"github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"
	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/distribution/reference"
	"github.com/dustin/go-humanize"
	"github.com/opencontainers/go-digest"
	imagespecs "github.com/opencontainers/image-spec/specs-go"
	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type InsecureOCIRegistryKind uint8

const (
	NoInsecureOCIRegistry InsecureOCIRegistryKind = iota
	SkipTLSVerifyOCIRegistry
	PlainHTTPOCIRegistry
)

type RewriteRefFunc func(sourceRef reference.Named) (targetRef reference.Named)

type OCIArtifactsBundler struct {
	Log                   logrus.FieldLogger
	InsecureRegistries    InsecureOCIRegistryKind
	RegistriesConfigPaths []string // uses the standard Docker config if empty
	PlatformMatcher       platforms.MatchComparer
	RewriteTarget         RewriteRefFunc

	// Limits the maximum number of concurrent artifact copy tasks.
	// Uses the ORAS default if zero.
	Concurrency uint
}

func (b *OCIArtifactsBundler) Run(ctx context.Context, refs []reference.Named, out io.Writer) error {
	var client *http.Client
	if len := len(refs); len < 1 {
		b.Log.Warn("No artifacts to bundle")
	} else {
		b.Log.Infof("About to bundle %d artifacts", len)
		var close func()
		client, close = newHttpClient(b.InsecureRegistries == SkipTLSVerifyOCIRegistry)
		defer close()
	}

	creds, err := newOCICredentials(b.RegistriesConfigPaths)
	if err != nil {
		return err
	}

	copyOpts := oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			Concurrency:    int(min(math.MaxInt, b.Concurrency)),
			FindSuccessors: findSuccessors(b.PlatformMatcher),
			PreCopy: func(ctx context.Context, desc imagespecv1.Descriptor) error {
				if desc.MediaType == images.MediaTypeDockerSchema1Manifest {
					// ORAS won't handle this on its own.
					return fmt.Errorf("cannot fetch %s: Docker Image Format v1 is unsupported", desc.Digest)
				}

				log := k0scontext.ValueOr(ctx, b.Log)
				log = log.WithFields(logrus.Fields{"mediaType": desc.MediaType, "digest": desc.Digest})
				if desc.Platform != nil {
					log = log.WithField("platform", platforms.FormatAll(*desc.Platform))
				}
				log.Info("Fetching ", humanize.IBytes(uint64(desc.Size)))
				return nil
			},
		},
	}

	tarWriter := tar.NewWriter(out)
	target := ociLayoutArchive{w: &ociLayoutArchiveWriter{tar: tarWriter}}
	index := imagespecv1.Index{
		Versioned: imagespecs.Versioned{SchemaVersion: 2},
		MediaType: imagespecv1.MediaTypeImageIndex,
	}

	for numRef, ref := range refs {
		ref := reference.TagNameOnly(ref)
		log := b.Log.WithFields(logrus.Fields{
			"artifact": fmt.Sprintf("%d/%d", numRef+1, len(refs)),
			"name":     ref,
		})
		ctx = k0scontext.WithValue[logrus.FieldLogger](ctx, log)

		source := remote.Repository{
			Client: &auth.Client{
				Client:     client,
				Credential: creds,
			},
			Reference: registry.Reference{
				Registry:   reference.Domain(ref),
				Repository: reference.Path(ref),
			},
			PlainHTTP: b.InsecureRegistries == PlainHTTPOCIRegistry,
		}

		desc, err := copyArtifact(ctx, ref, &source, &target, copyOpts)
		if err != nil {
			return fmt.Errorf("failed to bundle %s: %w", ref, err)
		}

		// Store the artifact multiple times with all its possible names.
		manifests, err := b.manifestsForRef(log, ref, desc)
		if err != nil {
			return fmt.Errorf("failed to bundle %s: %w", ref, err)
		}

		index.Manifests = append(index.Manifests, manifests...)
	}

	if err := writeTarJSON(tarWriter, imagespecv1.ImageIndexFile, 0644, index); err != nil {
		return err
	}
	if err := writeTarJSON(tarWriter, imagespecv1.ImageLayoutFile, 0444, &imagespecv1.ImageLayout{
		Version: imagespecv1.ImageLayoutVersion,
	}); err != nil {
		return err
	}

	return tarWriter.Close()
}

func copyArtifact(ctx context.Context, ref reference.Named, source oras.ReadOnlyTarget, target oras.Target, copyOpts oras.CopyOptions) (imagespecv1.Descriptor, error) {
	var srcRef string
	if tagged, ok := reference.TagNameOnly(ref).(reference.Tagged); ok {
		srcRef = tagged.Tag()
	}
	if digested, ok := ref.(reference.Digested); ok {
		expectedDigest := digested.Digest()
		if srcRef == "" {
			srcRef = expectedDigest.String()
		} else {
			// Pull via tag, but ensure that it matches the digest!
			copyOpts.MapRoot = func(_ context.Context, _ content.ReadOnlyStorage, root imagespecv1.Descriptor) (d imagespecv1.Descriptor, _ error) {
				if root.Digest == expectedDigest {
					return root, nil
				}
				return d, fmt.Errorf("%w for %s: %s", content.ErrMismatchedDigest, ref, root.Digest)
			}
		}
	}

	return oras.Copy(ctx, source, srcRef, target, "", copyOpts)
}

func (b *OCIArtifactsBundler) manifestsForRef(log *logrus.Entry, ref reference.Named, desc imagespecv1.Descriptor) ([]imagespecv1.Descriptor, error) {
	targetRef := ref
	if b.RewriteTarget != nil {
		targetRef = b.RewriteTarget(ref)
	}
	targetRefNames, err := targetRefNamesFor(targetRef)
	if err != nil {
		return nil, err
	}

	var manifests []imagespecv1.Descriptor
	for _, name := range targetRefNames {
		log.WithField("digest", desc.Digest).Info("Tagging ", name)
		desc := desc // shallow copy
		desc.Annotations = maps.Clone(desc.Annotations)
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string, 1)
		}
		desc.Annotations[imagespecv1.AnnotationRefName] = name
		manifests = append(manifests, desc)
	}
	return manifests, nil
}

func newHttpClient(insecureSkipTLSVerify bool) (_ *http.Client, close func()) {
	// This transports is, by design, a trimmed down version of http's DefaultTransport.
	// No need to have all those timeouts the default client brings in.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}

	if insecureSkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &http.Client{Transport: transport}, transport.CloseIdleConnections
}

func newOCICredentials(configPaths []string) (_ auth.CredentialFunc, err error) {
	var store credentials.Store
	var opts credentials.StoreOptions

	if len(configPaths) < 1 {
		store, err = credentials.NewStoreFromDocker(opts)
		if err != nil {
			return nil, err
		}
	} else {
		store, err = credentials.NewStore(configPaths[0], opts)
		if err != nil {
			return nil, err
		}
		if configPaths := configPaths[1:]; len(configPaths) > 0 {
			otherStores := make([]credentials.Store, len(configPaths))
			for i, path := range configPaths {
				otherStores[i], err = credentials.NewStore(path, opts)
				if err != nil {
					return nil, err
				}
			}
			store = credentials.NewStoreWithFallbacks(store, otherStores...)
		}
	}

	return credentials.Credential(store), nil
}

// Implement custom platform filtering. The default
// [oras.CopyOptions.WithTargetPlatform] will throw away multi-arch image
// indexes and thus change artifact digests.
func findSuccessors(platformMatcher platforms.MatchComparer) func(context.Context, content.Fetcher, imagespecv1.Descriptor) ([]imagespecv1.Descriptor, error) {
	if platformMatcher == nil {
		platformMatcher = platforms.Default()
	}
	return func(ctx context.Context, fetcher content.Fetcher, desc imagespecv1.Descriptor) ([]imagespecv1.Descriptor, error) {
		descs, err := content.Successors(ctx, fetcher, desc)
		if err != nil {
			return nil, err
		}

		var selectedDescs, discardedDescs []imagespecv1.Descriptor
		for _, desc := range descs {
			if desc.Platform == nil || platformMatcher.Match(*desc.Platform) {
				selectedDescs = append(selectedDescs, desc)
			} else {
				discardedDescs = append(discardedDescs, desc)
			}
		}
		retainBestPlatformOnly(&selectedDescs, platformMatcher.Less)

		// Fail if all images have been filtered out. This check has to happen
		// before the referencing descriptors get re-added, since they might
		// also be images (e.g. the attestation manifests).
		if slices.ContainsFunc(descs, isImage) && !slices.ContainsFunc(selectedDescs, isImage) {
			return nil, fmt.Errorf("%s: none of the available images match the requested platform", desc.Digest)
		}

		// Include descriptors that are referencing a previously selected digest.
		// Mostly to include Attestation Manifests.
		// https://github.com/moby/buildkit/blob/v0.27.1/docs/attestations/attestation-storage.md#attestation-manifest-descriptor
		for _, desc := range discardedDescs {
			refDigestAnnotation, ok := desc.Annotations["vnd.docker.reference.digest"]
			if !ok {
				continue
			}
			refDigest, err := digest.Parse(refDigestAnnotation)
			if err != nil {
				continue
			}
			if slices.ContainsFunc(selectedDescs, func(desc imagespecv1.Descriptor) bool { return desc.Digest == refDigest }) {
				selectedDescs = append(selectedDescs, desc)
			}
		}

		return selectedDescs, nil
	}
}

func retainBestPlatformOnly(descs *[]imagespecv1.Descriptor, isBetter func(imagespecv1.Platform, imagespecv1.Platform) bool) {
	// Sort the descriptors: The ones without platform first,
	// then the ones with platforms, better first.
	slices.SortFunc(*descs, func(l, r imagespecv1.Descriptor) int {
		lp, rp := l.Platform, r.Platform
		switch {
		case lp == nil:
			if rp == nil {
				return 0
			}
			return -1
		case rp == nil:
			return 1
		case isBetter(*lp, *rp):
			return -1
		case isBetter(*rp, *lp):
			return 1
		default:
			return 0
		}
	})

	// Truncate the descriptors: Retain all platformless descriptors,
	// plus the first (best) one with a platform.
	bestIdx := slices.IndexFunc(*descs, func(d imagespecv1.Descriptor) bool { return d.Platform != nil })
	if bestIdx >= 0 {
		*descs = (*descs)[:bestIdx+1]
	}
}

func isImage(desc imagespecv1.Descriptor) bool {
	switch desc.MediaType {
	case imagespecv1.MediaTypeImageManifest, "application/vnd.docker.distribution.manifest.v2+json":
		return true
	}
	return false
}

// Calculates the target references for the given input reference.
func targetRefNamesFor(ref reference.Named) (targetRefs []string, _ error) {
	// First the name as is, if it's not _only_ the name
	if !reference.IsNameOnly(ref) {
		targetRefs = append(targetRefs, ref.String())
	}

	nameOnly := reference.TrimNamed(ref)

	// Then as name:tag
	if tagged, ok := ref.(reference.Tagged); ok {
		tagged, err := reference.WithTag(nameOnly, tagged.Tag())
		if err != nil {
			return nil, err
		}
		targetRefs = append(targetRefs, tagged.String())
	}

	// Then as name@digest
	if digested, ok := ref.(reference.Digested); ok {
		digested, err := reference.WithDigest(nameOnly, digested.Digest())
		if err != nil {
			return nil, err
		}
		targetRefs = append(targetRefs, digested.String())
	}

	// Dedup the refs
	return stringslice.Unique(targetRefs), nil
}

func writeTarDir(w *tar.Writer, name string) error {
	return w.WriteHeader(&tar.Header{
		Name:     name + "/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	})
}

func writeTarJSON(w *tar.Writer, name string, mode os.FileMode, data any) error {
	json, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return writeTarFile(w, name, mode, int64(len(json)), bytes.NewReader(json))
}

func writeTarFile(w *tar.Writer, name string, mode os.FileMode, size int64, in io.Reader) error {
	if err := w.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     int64(mode),
		Size:     size,
	}); err != nil {
		return err
	}

	_, err := io.Copy(w, in)
	return err
}

type ociLayoutArchive struct {
	mu sync.RWMutex
	w  *ociLayoutArchiveWriter
}

type ociLayoutArchiveWriter struct {
	tar   *tar.Writer
	blobs []digest.Digest
}

func (t *ociLayoutArchive) doSynchronized(exclusive bool, fn func(w *ociLayoutArchiveWriter) error) (err error) {
	if exclusive {
		t.mu.Lock()
		defer func() {
			if err != nil {
				t.w = nil
			}
			t.mu.Unlock()
		}()
	} else {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}

	if t.w == nil {
		return errors.New("writer is broken")
	}

	return fn(t.w)
}

// Exists implements [oras.Target].
func (a *ociLayoutArchive) Exists(ctx context.Context, target imagespecv1.Descriptor) (exists bool, _ error) {
	err := a.doSynchronized(false, func(w *ociLayoutArchiveWriter) error {
		_, exists = slices.BinarySearch(w.blobs, target.Digest)
		return nil
	})
	return exists, err
}

// Push implements [oras.Target].
func (a *ociLayoutArchive) Push(ctx context.Context, expected imagespecv1.Descriptor, in io.Reader) (err error) {
	d := expected.Digest
	if err := d.Validate(); err != nil {
		return err
	}

	lockErr := a.doSynchronized(true, func(w *ociLayoutArchiveWriter) error {
		idx, exists := slices.BinarySearch(w.blobs, d)
		if exists {
			err = errdef.ErrAlreadyExists
			return nil
		}

		if len(w.blobs) < 1 {
			if err := writeTarDir(w.tar, imagespecv1.ImageBlobsDir); err != nil {
				return err
			}
		}

		if (idx == 0 || w.blobs[idx-1].Algorithm() != d.Algorithm()) &&
			(idx >= len(w.blobs) || w.blobs[idx].Algorithm() != d.Algorithm()) {
			dirName := path.Join(imagespecv1.ImageBlobsDir, d.Algorithm().String())
			if err := writeTarDir(w.tar, dirName); err != nil {
				return err
			}
		}

		blobName := path.Join(imagespecv1.ImageBlobsDir, d.Algorithm().String(), d.Hex())
		verify := content.NewVerifyReader(in, expected)
		if err := writeTarFile(w.tar, blobName, 0444, expected.Size, verify); err != nil {
			return err
		}
		if err := verify.Verify(); err != nil {
			return err
		}

		w.blobs = slices.Insert(w.blobs, idx, d)
		return nil
	})

	return cmp.Or(lockErr, err)
}

// Tag implements [oras.Target].
func (a *ociLayoutArchive) Tag(ctx context.Context, desc imagespecv1.Descriptor, reference string) error {
	if exists, err := a.Exists(ctx, desc); err != nil {
		return err
	} else if !exists {
		return errdef.ErrNotFound
	}

	return nil // don't store tag information
}

// Resolve implements [oras.Target].
func (a *ociLayoutArchive) Resolve(ctx context.Context, reference string) (d imagespecv1.Descriptor, _ error) {
	return d, fmt.Errorf("%w: Resolve(_, %q)", errdef.ErrUnsupported, reference)
}

// Fetch implements [oras.Target].
func (a *ociLayoutArchive) Fetch(ctx context.Context, target imagespecv1.Descriptor) (io.ReadCloser, error) {
	return nil, fmt.Errorf("%w: Fetch(_, %v)", errdef.ErrUnsupported, target)
}
