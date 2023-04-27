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

package airgap

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/iotest"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAirgapListImages(t *testing.T) {
	// TODO: k0s will always try to read the runtime config file first
	// (/run/k0s/k0s.yaml). There's currently no knob to change that (maybe use
	// XDG_RUNTIME_DIR, XDG_STATE_HOME, XDG_DATA_HOME?). If the file is present
	// on a host executing this test, it will interfere with it.
	require.NoFileExists(t, "/run/k0s/k0s.yaml", "Runtime config exists and will interfere with this test.")

	defaultImage := v1beta1.DefaultEnvoyProxyImage().URI()

	t.Run("All", func(t *testing.T) {
		underTest, out, err := newAirgapListImagesCmdWithConfig(t, "{}", "--all")

		require.NoError(t, underTest.Execute())
		lines := intoLines(out)
		if runtime.GOARCH == "arm" {
			assert.NotContains(t, lines, defaultImage)
		} else {
			assert.Contains(t, lines, defaultImage)
		}

		assert.Empty(t, err.String())
	})

	t.Run("NodeLocalLoadBalancing", func(t *testing.T) {
		const (
			customImage = "example.com/envoy:v1337"
			yamlData    = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  network:
    nodeLocalLoadBalancing:
      enabled: %t
      envoyProxy:
        image:
          image: example.com/envoy
          version: v1337`
		)

		for _, test := range []struct {
			name                    string
			enabled                 bool
			contained, notContained []string
		}{
			{"enabled", true, []string{customImage}, []string{defaultImage}},
			{"disabled", false, nil, []string{customImage, defaultImage}},
		} {
			t.Run(test.name, func(t *testing.T) {
				underTest, out, err := newAirgapListImagesCmdWithConfig(t, fmt.Sprintf(yamlData, test.enabled))

				require.NoError(t, underTest.Execute())

				lines := intoLines(out)
				for _, contained := range test.contained {
					if runtime.GOARCH == "arm" {
						assert.NotContains(t, lines, contained)
					} else {
						assert.Contains(t, lines, contained)
					}
				}
				for _, notContained := range test.notContained {
					assert.NotContains(t, lines, notContained)
				}
				assert.Empty(t, err.String())
			})
		}
	})
}

func newAirgapListImagesCmdWithConfig(t *testing.T, config string, args ...string) (_ *cobra.Command, out, err *bytes.Buffer) {
	configFile := filepath.Join(t.TempDir(), "k0s.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0644))

	out, err = new(bytes.Buffer), new(bytes.Buffer)
	cmd := NewAirgapListImagesCmd()
	cmd.SetArgs(append([]string{"--config=" + configFile}, args...))
	cmd.SetIn(iotest.ErrReader(errors.New("unexpected read from standard input")))
	cmd.SetOut(out)
	cmd.SetErr(err)
	return cmd, out, err
}

func intoLines(in io.Reader) (lines []string) {
	scanner := bufio.NewScanner(in)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return
}
