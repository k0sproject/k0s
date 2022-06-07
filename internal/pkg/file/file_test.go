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
package file

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	dir := t.TempDir()

	// test no-existing
	got := Exists(filepath.Join(dir, "non-existing"))
	want := false
	if got != want {
		t.Errorf("test non-existing: got %t, wanted %t", got, want)
	}

	existingFileName := path.Join(dir, "existing")
	require.NoError(t, os.WriteFile(existingFileName, []byte{}, 0644))

	// test existing
	got = Exists(existingFileName)
	want = true
	if got != want {
		t.Errorf("test existing tempfile %s: got %t, wanted %t", existingFileName, got, want)
	}

	// test what happens when we don't have permissions to the directory to file
	// and can confirm that it actually exists
	if assert.NoError(t, os.Chmod(dir, 0000)) {
		t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	}

	got = Exists(existingFileName)
	want = false
	if got != want {
		t.Errorf("test existing tempfile %s: got %t, wanted %t", existingFileName, got, want)
	}

}
