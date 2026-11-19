//go:build !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"

	"github.com/spf13/pflag"
)

type AppArmorFlags struct{}

func (*AppArmorFlags) AddToFlagSet(*pflag.FlagSet)                     {}
func (*AppArmorFlags) ContainerdConfig() (_ containerd.AppArmorConfig) { return }
