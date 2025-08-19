// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	internalio "github.com/k0sproject/k0s/internal/io"

	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleArtifactsCmd_RejectsCertificate(t *testing.T) {
	t.Parallel()

	log := logrus.New()
	log.Out = io.Discard
	var stderr strings.Builder

	registry := startFakeRegistry(t, false)

	underTest := newAirgapBundleArtifactsCmd(log, nil)
	underTest.SetIn(strings.NewReader(path.Join(registry, "hello:1980")))
	underTest.SetOut(internalio.WriterFunc(func(d []byte) (int, error) {
		assert.Fail(t, "Expected no writes to standard output", "Written: %s", d)
		return 0, assert.AnError
	}))
	underTest.SetErr(&stderr)

	err := underTest.Execute()

	expected := "tls: failed to verify certificate: x509: certificate signed by unknown authority"
	assert.ErrorContains(t, err, registry)
	assert.ErrorContains(t, err, expected)
	assert.Contains(t, stderr.String(), expected)
}

func TestBundleArtifactsCmd_WithPlatforms(t *testing.T) {
	log := logrus.New()
	log.Out = io.Discard

	for _, insecureRegistriesFlag := range []string{"skip-tls-verify", "plain-http"} {
		t.Run(insecureRegistriesFlag, func(t *testing.T) {
			registry := startFakeRegistry(t, insecureRegistriesFlag == "plain-http")
			ref := registry + "/hello:1980"

			// Need to rewrite the artifact name to get reproducible output.
			rewriteBundleRef := func(sourceRef reference.Named) (targetRef reference.Named) {
				if sourceRef.String() == ref {
					targetRef, err := reference.ParseNamed("registry.example.com/hello:1980")
					if assert.NoError(t, err) {
						return targetRef
					}
				}
				return sourceRef
			}

			for platform, digest := range map[string]string{
				"linux/amd64":  "7c7a6255a6bdf5ae9cb5e717852a34180b124dc15ba29e1f922459613c206e68",
				"linux/arm64":  "ae78a79237689e234ba5272130a9739ae64fe9df349aee363b4491fd98cb5cf1",
				"linux/arm/v7": "44e355bbfb4c874b28aa6e6773481d2f64bc03d37aa793a988209a6bd5911a6d",
			} {
				t.Run(platform, func(t *testing.T) {
					hasher := sha256.New()
					underTest := newAirgapBundleArtifactsCmd(log, rewriteBundleRef)
					underTest.SetArgs([]string{
						"--insecure-registries", insecureRegistriesFlag,
						"--platform", platform,
						"--concurrency", "1", // reproducible output
						ref,
					})
					underTest.SetIn(iotest.ErrReader(errors.New("unexpected read from standard input")))
					underTest.SetOut(hasher)
					underTest.SetErr(internalio.WriterFunc(func(d []byte) (int, error) {
						assert.Fail(t, "Expected no writes to standard error", "Written: %s", d)
						return 0, assert.AnError
					}))

					require.NoError(t, underTest.Execute())
					assert.Equal(t, digest, hex.EncodeToString(hasher.Sum(nil)))
				})
			}
		})
	}
}

func startFakeRegistry(t *testing.T, plainHTTP bool) string {
	manifests := make(map[string]digest.Digest)
	var contentTypes map[digest.Digest]string
	if data, err := os.ReadFile(filepath.Join("testdata", "oci-layout", imagespecv1.ImageIndexFile)); assert.NoError(t, err) {
		var index imagespecv1.Index
		require.NoError(t, json.Unmarshal(data, &index))
		for _, manifest := range index.Manifests {
			name := manifest.Annotations[imagespecv1.AnnotationRefName]
			if name != "" {
				manifests[name] = manifest.Digest
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join("testdata", "oci-layout", "content-types.json")); assert.NoError(t, err) {
		require.NoError(t, json.Unmarshal(data, &contentTypes))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/{name}/{kind}/{ident}", func(w http.ResponseWriter, r *http.Request) {
		var dgst digest.Digest
		switch r.PathValue("kind") {
		case "manifests":
			var found bool
			name := r.PathValue("name") + ":" + r.PathValue("ident")
			if dgst, found = manifests[name]; found {
				break
			}
			fallthrough
		case "blobs":
			dgst = digest.Digest(r.PathValue("ident"))
			if err := dgst.Validate(); err == nil {
				break
			}
			fallthrough
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", contentTypes[dgst])
		path := filepath.Join("testdata", "oci-layout", "blobs", dgst.Algorithm().String(), dgst.Hex())
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			assert.NoError(t, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = w.Write(data)
		assert.NoError(t, err)
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Proto, r.Method, r.RequestURI)
		mux.ServeHTTP(w, r)
	})

	var server *httptest.Server
	if plainHTTP {
		server = httptest.NewServer(handler)
	} else {
		server = httptest.NewTLSServer(handler)
	}
	t.Cleanup(server.Close)

	url, err := url.Parse(server.URL)
	require.NoError(t, err)
	return path.Join(url.Host, url.Path)
}
