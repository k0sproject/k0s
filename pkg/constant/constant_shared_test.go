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
package constant

import (
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKonnectivityVersion(t *testing.T) {
	assert.Equal(t, getVersion(t, "konnectivity"), strings.TrimPrefix(KonnectivityImageVersion, "v"))
}

func TestKubeProxyVersion(t *testing.T) {
	assert.Equal(t, getVersion(t, "kubernetes"), strings.TrimPrefix(KubeProxyImageVersion, "v"))
}

func TestKubernetesMajorMinorVersion(t *testing.T) {
	ver := strings.Split(getVersion(t, "kubernetes"), ".")
	require.GreaterOrEqual(t, len(ver), 2, "failed to spilt Kubernetes version %q", ver)
	kubeMinorMajor := ver[0] + "." + ver[1]

	assert.Equal(t, kubeMinorMajor, KubernetesMajorMinorVersion)
}

func getVersion(t *testing.T, component string) string {
	cmd := exec.Command("make", "--no-print-directory", "-s", "-f", "vars.mk", component+"_version")
	cmd.Dir = path.Join("..", "..")

	out, err := cmd.Output()
	require.NoError(t, err)
	require.NotEmpty(t, out, "failed to get %s version", component)

	return strings.TrimSuffix(string(out), "\n")
}
