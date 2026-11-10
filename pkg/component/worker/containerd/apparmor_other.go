//go:build !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"

	"github.com/k0sproject/k0s/pkg/supervisor"
)

func DumpContainerAppArmorDefaultProfileHook() {}

type AppArmorConfig struct{}

func (c *Component) setupAppArmor(context.Context) (_ supervisor.ProcessStarter, _ error) { return }
