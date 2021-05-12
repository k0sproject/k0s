// +build !linux

package telemetry

import (
	"runtime"

	"github.com/segmentio/analytics-go"
)

func addSysInfo(d *analytics.Context) {
	d.OS.Name = runtime.GOOS
}
