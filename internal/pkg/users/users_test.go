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

package users

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("No numeric user IDs on Windows")
	}

	uid, err := LookupUID("root")
	if assert.NoError(t, err, "Failed to get UID for root user") {
		assert.Equal(t, 0, uid, "root's UID is not 0?")
	}

	uid, err = LookupUID("some-non-existing-user")
	if assert.Error(t, err, "Got a UID for some-non-existing-user?") {
		assert.ErrorIs(t, err, ErrNotExist)
		var exitErr *exec.ExitError
		assert.ErrorAs(t, err, &exitErr, "expected external `id` to return an error")
		assert.Equal(t, UnknownUID, uid)
	}
}
