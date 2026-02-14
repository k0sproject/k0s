// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
)

// ContextBackground returns context.Background so that it can be used without triggering
// the contextcheck linter in t.Cleanup.
func ContextBackground() context.Context {
	return context.Background()
}
