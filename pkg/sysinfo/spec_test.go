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

package sysinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	system "k8s.io/system-validators/validators"
)

func TestSpecSanity(t *testing.T) {

	for _, test := range []struct {
		name string
		spec K0sSpec
	}{
		{name: "preflightSpec", spec: preflightSpec()},
		{name: "sysinfoSpec", spec: sysinfoSpec()},
	} {
		t.Run(test.name, func(t *testing.T) {
			allKernelConfigs := [][]system.KernelConfig{
				test.spec.sys.KernelSpec.Required,
				test.spec.sys.KernelSpec.Optional,
				test.spec.sys.KernelSpec.Forbidden,
			}

			t.Run("noDuplicateKernelConfigs", func(t *testing.T) {
				seen := make(map[string]struct{})

				for _, configs := range allKernelConfigs {
					for _, config := range configs {
						_, present := seen[config.Name]
						assert.False(t, present, "duplicate config in kernel spec", config.Name)
						seen[config.Name] = struct{}{}
					}
				}
			})

			t.Run("noAufsInKernelSpec", func(t *testing.T) {
				for _, configs := range allKernelConfigs {
					for _, config := range configs {
						assert.NotEqual(t, "AUFS_FS", config.Name)
					}
				}
			})
		})
	}
}
