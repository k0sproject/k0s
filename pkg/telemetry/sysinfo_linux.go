// +build linux

package telemetry

import (
	"os"

	"github.com/segmentio/analytics-go"
	"github.com/zcalusic/sysinfo"
)

func addSysInfo(d *analytics.Context) {
	var si sysinfo.SysInfo
	si.GetSysInfo()

	d.OS.Name = si.OS.Name
	d.OS.Version = si.OS.Version

	d.Extra["cpuCount"] = si.CPU.Cpus
	d.Extra["cpuCores"] = si.CPU.Cores
	d.Extra["memTotal"] = si.Memory.Size
	d.Extra["haveProxy"] = os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != ""
}
