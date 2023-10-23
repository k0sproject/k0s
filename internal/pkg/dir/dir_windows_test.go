/*
Copyright 2023 k0s authors

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

package dir_test

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"

	"github.com/stretchr/testify/require"
)

func TestPathListJoin(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.Equal(t, "", dir.PathListJoin())
	})
	t.Run("single", func(t *testing.T) {
		require.Equal(t, "foo", dir.PathListJoin("foo"))
	})
	t.Run("multiple", func(t *testing.T) {
		require.Equal(t, "foo;bar;baz", dir.PathListJoin("foo", "bar", "baz"))
	})
}
