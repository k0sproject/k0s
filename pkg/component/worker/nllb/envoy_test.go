/*
Copyright 2022 k0s authors

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

package nllb

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	k0snet "github.com/k0sproject/k0s/internal/pkg/net"

	"k8s.io/client-go/util/jsonpath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestWriteEnvoyConfigFiles(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected int
		servers  []string
	}{
		{"empty", 0, []string{}},
		{"one", 1, []string{"foo:16"}},
		{"two", 2, []string{"foo:16", "bar:17"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			params := envoyParams{
				configDir: dir,
				bindIP:    net.IPv6loopback,
			}
			filesParams := envoyFilesParams{}
			for _, server := range test.servers {
				server, err := k0snet.ParseHostPort(server)
				require.NoError(t, err)
				filesParams.apiServers = append(filesParams.apiServers, *server)
			}

			require.NoError(t, writeEnvoyConfigFiles(&params, &filesParams))

			parse := func(t *testing.T, file string) (parsed map[string]any) {
				content, err := os.ReadFile(filepath.Join(dir, file))
				require.NoError(t, err)
				require.NoError(t, yaml.Unmarshal(content, &parsed), "invalid YAML in %s", file)
				return
			}

			t.Run("envoy.yaml", func(t *testing.T) {
				ip, err := evalJSONPath[string](parse(t, "envoy.yaml"),
					".static_resources.listeners[0].address.socket_address.address",
				)
				require.NoError(t, err)
				assert.Equal(t, "::1", ip)
			})

			t.Run("cds.yaml", func(t *testing.T) {
				eps, err := evalJSONPath[[]any](parse(t, "cds.yaml"),
					".resources[0].load_assignment.endpoints[0].lb_endpoints",
				)
				require.NoError(t, err)

				addrs := []string{}
				for i, ep := range eps {
					host, herr := evalJSONPath[string](ep, ".endpoint.address.socket_address.address")
					port, perr := evalJSONPath[float64](ep, ".endpoint.address.socket_address.port_value")
					if assert.NoError(t, errors.Join(herr, perr), "For endpoint %d", i) {
						iport := int64(port)
						if assert.Equal(t, float64(iport), port, "Port is not an integer for endpoint %d", i) {
							addrs = append(addrs, fmt.Sprintf("%s:%d", host, iport))
						}
					}
				}
				if !t.Failed() {
					assert.Equal(t, test.servers, addrs)
				}
			})
		})
	}
}

func evalJSONPath[T any](json any, path string) (t T, _ error) {
	tpl := jsonpath.New("")
	if err := tpl.Parse("{" + path + "}"); err != nil {
		return t, err
	}

	results, err := tpl.FindResults(json)
	switch {
	case err != nil:
		return t, err
	case len(results) == 0:
		return t, errors.New("given jsonpath expression does not match any value")
	case len(results) > 1:
		return t, errors.New("given jsonpath expression matches more than one list")
	case len(results[0]) > 1:
		return t, errors.New("given jsonpath expression matches more than one value")
	}

	candidate := results[0][0].Interface()
	converted, ok := candidate.(T)
	if !ok {
		return t, fmt.Errorf("expected %T, found %T", t, candidate)
	}

	return converted, nil
}
