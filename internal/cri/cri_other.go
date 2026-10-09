//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"net/url"
)

func newCRIRuntime(runtimeEndpoint *url.URL) *CRIRuntime {
	return &CRIRuntime{
		target:      runtimeEndpoint.String(),
		dialOptions: defaultGRPCDialOptions,
	}
}
