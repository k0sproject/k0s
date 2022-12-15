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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestAvailableComponents_SortedAndUnique(t *testing.T) {
	expected := slices.Clone(availableComponents)
	slices.Sort(expected)

	assert.Equal(t, expected, availableComponents, "Available components aren't sorted")

	slices.Compact(expected)
	assert.Equal(t, expected, availableComponents, "Available components contain duplicates")
}

func TestControllerOptions_Normalize(t *testing.T) {
	t.Run("failsOnUnknownComponent", func(t *testing.T) {
		disabled := []string{"i-dont-exist"}

		underTest := ControllerOptions{DisableComponents: disabled}
		err := underTest.Normalize()

		assert.ErrorContains(t, err, "unknown component i-dont-exist")
	})

	for _, test := range []struct {
		name               string
		disabled, expected []string
	}{
		{
			"removesDuplicateComponents",
			[]string{"helm", "kube-proxy", "coredns", "kube-proxy", "autopilot"},
			[]string{"helm", "kube-proxy", "coredns", "autopilot"},
		},
		{
			"replacesDeprecation",
			[]string{"helm", "kubelet-config", "coredns", "kubelet-config", "autopilot"},
			[]string{"helm", "worker-config", "coredns", "autopilot"},
		},
		{
			"replacesDeprecationAvoidingDuplicates",
			[]string{"helm", "kubelet-config", "coredns", "kubelet-config", "worker-config", "autopilot"},
			[]string{"helm", "worker-config", "coredns", "autopilot"},
		},
	} {
		underTest := ControllerOptions{DisableComponents: test.disabled}
		err := underTest.Normalize()

		require.NoError(t, err)
		assert.Equal(t, test.expected, underTest.DisableComponents)
	}
}
