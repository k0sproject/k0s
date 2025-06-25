// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
)

// PlatformIdentifier returns a consistent string identifier representing
// the OS and architecture of the current machine.
func PlatformIdentifier(os, arch string) string {
	return fmt.Sprintf("%s-%s", os, arch)
}
