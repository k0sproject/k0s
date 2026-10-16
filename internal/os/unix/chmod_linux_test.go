// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix_test

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestPath_Chmod_OPath(t *testing.T) {
	testPathChmod(t, unix.O_PATH)
}
