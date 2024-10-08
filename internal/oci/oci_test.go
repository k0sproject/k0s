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

package oci

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
		fpath := fmt.Sprintf("testdata/%s", entry.Name())
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
	Name          string
	Manifest      string            `yaml:"manifest"`
	Expected      string            `yaml:"expected"`
	Error         string            `yaml:"error"`
	Authenticated bool              `yaml:"authenticated"`
	AuthUser      string            `yaml:"authUser"`
	AuthPass      string            `yaml:"authPass"`
	Artifacts     map[string]string `yaml:"artifacts"`
}

func TestDownload(t *testing.T) {
	for tname, tt := range parseTestsYAML[testFile](t) {
		t.Run(tname, func(t *testing.T) {
			addr := startOCIMockServer(t, tt)

			opts := []OrasOption{WithInsecureSkipTLSVerify()}
			if tt.Authenticated {
				entry := DockerConfigEntry{tt.AuthUser, tt.AuthPass}
				opts = append(opts, WithDockerAuth(
					DockerConfig{
						Auths: map[string]DockerConfigEntry{
							addr: entry,
						},
					},
				))
			}

			buf := bytes.NewBuffer(nil)
			url := fmt.Sprintf("%s/repository/artifact:latest", addr)
			err := Download(context.TODO(), url, buf, opts...)
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
func startOCIMockServer(t *testing.T, test testFile) string {
	var serverAddr string
	server := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// this is a request to authenticate.
			if strings.Contains(r.URL.Path, "/token") {
				user, pass, _ := r.BasicAuth()
				if user != "user" || pass != "pass" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = w.Write([]byte(`{"token":"token"}`))
				return
			}

			// verify if the request should be authenticated or
			// not. if it has already been authenticated then just
			// moves on.
			_, authenticated := r.Header["Authorization"]
			if !authenticated && test.Authenticated {
				header := fmt.Sprintf(`Bearer realm="https://%s/token"`, serverAddr)
				w.Header().Add("WWW-Authenticate", header)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// serve the manifest.
			if strings.Contains(r.URL.Path, "/manifests/") {
				w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				_, _ = w.Write([]byte(test.Manifest))
				return
			}

			// serve a layer or the config blob.
			if strings.Contains(r.URL.Path, "/blobs/") {
				for sha, content := range test.Artifacts {
					if !strings.Contains(r.URL.Path, sha) {
						continue
					}
					length := fmt.Sprintf("%d", len(content))
					w.Header().Add("Content-Length", length)
					_, _ = w.Write([]byte(content))
					return
				}
			}

			t.Fatalf("unexpected request: %s", r.URL.Path)
		}),
	)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	serverAddr = u.Host
	return serverAddr
}
