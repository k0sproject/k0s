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

	t.Run("removesDuplicateComponents", func(t *testing.T) {
		disabled := []string{"helm", "kube-proxy", "coredns", "kube-proxy", "autopilot"}
		expected := []string{"helm", "kube-proxy", "coredns", "autopilot"}

		underTest := ControllerOptions{DisableComponents: disabled}
		err := underTest.Normalize()

		require.NoError(t, err)
		assert.Equal(t, expected, underTest.DisableComponents)
	})
}
