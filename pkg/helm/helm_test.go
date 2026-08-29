// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestEffectiveTimeout covers the regression behind
// https://github.com/k0sproject/k0s/issues/8181: UninstallRelease used to
// derive its Helm action timeout solely from ctx.Deadline(). Callers such as
// the Chart reconciler never attach a deadline to ctx, so that timeout ended
// up as zero, and Helm's wait-for-delete step then failed immediately with
// "context deadline exceeded" instead of actually waiting for the release's
// resources to be removed. effectiveTimeout now takes an explicit timeout
// (mirroring InstallChart/UpgradeChart) and only shortens it when ctx
// carries a sooner deadline.
func TestEffectiveTimeout(t *testing.T) {
	t.Run("NoDeadline", func(t *testing.T) {
		got := effectiveTimeout(t.Context(), 7*time.Minute)
		assert.Equal(t, 7*time.Minute, got)
	})

	t.Run("DeadlineLaterThanTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
			defer cancel()

			got := effectiveTimeout(ctx, 5*time.Minute)
			assert.Equal(t, 5*time.Minute, got)
		})
	})

	t.Run("DeadlineSoonerThanTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			got := effectiveTimeout(ctx, 5*time.Minute)
			assert.Equal(t, 30*time.Second, got)
		})
	})
}
