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
package constant

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubeProxyVersion(t *testing.T) {
	kubeVersion := getKubeVersion(t)
	assert.Equal(t, kubeVersion, strings.TrimPrefix(KubeProxyImageVersion, "v"))
}

func TestKubernetesMajorMinorVersion(t *testing.T) {
	kubeVersion := getKubeVersion(t)

	ver := strings.Split(kubeVersion, ".")
	kubeMinorMajor := ver[0] + "." + ver[1]

	assert.Equal(t, kubeMinorMajor, KubernetesMajorMinorVersion)
}

func getKubeVersion(t *testing.T) string {
	cmd := exec.Command("make", "--no-print-directory", "-f", "-", "print-kubernetes_version")
	cmd.Stdin = bytes.NewBuffer([]byte(makefile))

	out, err := cmd.Output()
	assert.Nil(t, err)

	return strings.TrimSuffix(string(out), "\n")
}

const makefile = `
include ../../embedded-bins/Makefile.variables

print-kubernetes_version:
	@echo $(kubernetes_version)
`
