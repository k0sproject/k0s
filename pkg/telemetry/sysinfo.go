//go:build !linux

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"runtime"

	"github.com/segmentio/analytics-go/v3"
)

func addSysInfo(d *analytics.Context) {
	d.OS.Name = runtime.GOOS
}
