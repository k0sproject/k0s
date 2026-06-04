//go:build !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
)

func watchDir(ctx context.Context, w *dirWatch) error {
	err, _ := w.runFSNotify(ctx)
	return err
}
