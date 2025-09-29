// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
)

func addPlatformSpecificComponents(context.Context, *manager.Manager, *config.CfgVars, EmbeddingController, *worker.CertificateManager) {
	// no-op
}
