//go:build !windows

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
)

func defaultNodeNameOverride(context.Context) (string, error) {
	return "", nil // no default override
}
