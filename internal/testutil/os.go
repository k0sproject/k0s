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

package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Chdir changes the current working directory to the named directory and
// returns a function that, when called, restores the original working
// directory.
//
// Use in tests like so:
//
//	tmpDir := t.TempDir()
//	defer testutil.Chdir(t, tmpDir)()
func Chdir(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	return func() {
		require.NoError(t, os.Chdir(wd))
	}
}
