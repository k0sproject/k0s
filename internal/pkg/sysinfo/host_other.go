//go:build !linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysinfo

import (
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (s *K0sSysinfoSpec) addHostSpecificProbes(p probes.Probes) {
	// no-op
}
