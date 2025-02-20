//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervised

import (
	"context"
)

func run(main MainFunc) error {
	ctx, cancel := ShutdownContext(context.Background())
	defer cancel(nil)
	return main(ctx)
}
