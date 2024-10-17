/*
Copyright 2024 k0s authors

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

package oci_test

import (
	"bytes"
	"cmp"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/oci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

//go:embed testdata/*
var testData embed.FS

// we define our tests as yaml files inside the testdata directory. this
// function parses them and returns a map of the tests.
func parseTestsYAML[T any](t *testing.T) map[string]T {
	entries, err := testData.ReadDir("testdata")
	require.NoError(t, err)
	tests := make(map[string]T, 0)
	for _, entry := range entries {
		fpath := path.Join("testdata", entry.Name())
		data, err := testData.ReadFile(fpath)
		require.NoError(t, err)

		var onetest T
		err = yaml.Unmarshal(data, &onetest)
		require.NoError(t, err)

		tests[fpath] = onetest
	}
	return tests
}

// testFile represents a single test file inside the testdata directory.
type testFile struct {
	Manifest          string            `json:"manifest"`
	ManifestMediaType string            `json:"manifestMediaType"`
	Expected          string            `json:"expected"`
	Error             string            `json:"error"`
	Authenticated     bool              `json:"authenticated"`
	AuthUser          string            `json:"authUser"`
	AuthPass          string            `json:"authPass"`
	Artifacts         map[string]string `json:"artifacts"`
	ArtifactName      string            `json:"artifactName"`
	PlainHTTP         bool              `json:"plainHTTP"`
}

func TestDownload(t *testing.T) {
	for tname, tt := range parseTestsYAML[testFile](t) {
		t.Run(tname, func(t *testing.T) {
			addr := startOCIMockServer(t, tname, tt)

			opts := []oci.DownloadOption{oci.WithInsecureSkipTLSVerify()}
			if tt.Authenticated {
				entry := oci.DockerConfigEntry{tt.AuthUser, tt.AuthPass}
				opts = append(opts, oci.WithDockerAuth(
					oci.DockerConfig{
						Auths: map[string]oci.DockerConfigEntry{
							addr: entry,
						},
					},
				))
			}

			if tt.ArtifactName != "" {
				opts = append(opts, oci.WithArtifactName(tt.ArtifactName))
			}

			if tt.PlainHTTP {
				opts = append(opts, oci.WithPlainHTTP())
			}

			buf := bytes.NewBuffer(nil)
			url := fmt.Sprintf("%s/repository/artifact:latest", addr)
			err := oci.Download(context.TODO(), url, buf, opts...)
			if tt.Expected != "" {
				require.NoError(t, err)
				require.Empty(t, tt.Error)
				require.Equal(t, tt.Expected, buf.String())
				return
			}
			require.NotEmpty(t, tt.Error)
			require.ErrorContains(t, err, tt.Error)
		})
	}
}

// startOCIMockServer starts a mock server that will respond to the given test.
// this mimics the behavior of the real OCI registry. This function returns the
// address of the server.
func startOCIMockServer(t *testing.T, tname string, test testFile) string {
	var serverAddr string

	starter := httptest.NewTLSServer
	if test.PlainHTTP {
		starter = httptest.NewServer
	}

	server := starter(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Log(r.Proto, r.Method, r.RequestURI)
			if !assert.Equal(t, r.Method, http.MethodGet) {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// this is a request to authenticate.
			if strings.Contains(r.URL.Path, "/token") {
				user, pass, _ := r.BasicAuth()
				if user != "user" || pass != "pass" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				res := map[string]string{"token": tname}
				marshalled, err := json.Marshal(res)
				assert.NoError(t, err)
				_, _ = w.Write(marshalled)
				return
			}

			// verify if the request should be authenticated or
			// not. if it has already been authenticated then just
			// moves on. the token returned is the test name.
			tokenhdr, authenticated := r.Header["Authorization"]
			if !authenticated && test.Authenticated {
				proto := "https"
				if test.PlainHTTP {
					proto = "http"
				}

				header := fmt.Sprintf(`Bearer realm="%s://%s/token"`, proto, serverAddr)
				w.Header().Add("WWW-Authenticate", header)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// verify if the token provided by the client matches
			// the expected token.
			if test.Authenticated {
				assert.Len(t, tokenhdr, 1)
				assert.Contains(t, tokenhdr[0], tname)
			}

			// serve the manifest.
			if strings.Contains(r.URL.Path, "/manifests/") {
				w.Header().Add("Content-Type", cmp.Or(test.ManifestMediaType, "application/vnd.oci.image.manifest.v1+json"))
				_, _ = w.Write([]byte(test.Manifest))
				return
			}

			// serve a layer or the config blob.
			if strings.Contains(r.URL.Path, "/blobs/") {
				for sha, content := range test.Artifacts {
					if !strings.Contains(r.URL.Path, sha) {
						continue
					}
					w.Header().Add("Content-Length", strconv.Itoa(len(content)))
					_, _ = w.Write([]byte(content))
					return
				}
			}

			assert.Failf(t, "unexpected request", "%s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	serverAddr = u.Host
	return serverAddr
}
