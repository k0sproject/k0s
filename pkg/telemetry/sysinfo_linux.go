// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/k0sproject/k0s/internal/os/linux"

	"github.com/segmentio/analytics-go/v3"
)

func addSysInfo(d *analytics.Context) {
	totalMemory, _ := linux.TotalMemory() // zero if unknown

	d.OS.Name, d.OS.Version = osNameAndVersion(os.DirFS("/"))
	d.Extra["cpuCount"], d.Extra["cpuCores"] = countCPUsAndCores(os.DirFS("/proc"))
	d.Extra["memTotal"] = totalMemory / (1024 * 1024) // in MiB
	d.Extra["haveProxy"] = os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != ""
}

func osNameAndVersion(root fs.FS) (name, version string) {
	f, err := root.Open("etc/os-release")
	if errors.Is(err, fs.ErrNotExist) {
		// As per os-release(5), this is the fallback.
		f, err = root.Open("usr/lib/os-release")
	}
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	var id string
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		if k, v, ok := bytes.Cut(lines.Bytes(), []byte{'='}); ok {
			var target *string
			switch string(bytes.TrimSpace(k)) {
			case "PRETTY_NAME":
				target = &name
			case "ID":
				target = &id
			case "VERSION_ID":
				target = &version
			default:
				continue
			}
			*target = string(bytes.Trim(bytes.TrimSpace(v), `"`))
		}
	}
	if lines.Err() != nil {
		return name, version
	}

	var releaseFile, releasePrefix string
	switch id {
	case "almalinux":
		releaseFile, releasePrefix = "etc/almalinux-release", "AlmaLinux release "
	case "rocky":
		releaseFile, releasePrefix = "etc/rocky-release", "Rocky Linux release "
	default:
		return name, version
	}

	if data, err := fs.ReadFile(root, releaseFile); err == nil {
		if rest, ok := strings.CutPrefix(string(data), releasePrefix); ok {
			if release := rest[:len(rest)-len(strings.TrimLeft(rest, "0123456789."))]; release != "" {
				version = release
			}
		}
	}

	version, _, _ = strings.Cut(version, ".")

	return name, version
}

func countCPUsAndCores(procFS fs.FS) (cpus, cores uint) {
	f, err := procFS.Open("cpuinfo")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()

	var (
		cpuIDs, coreIDs []string
		currentCPUID    string
	)
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		if key, value, found := bytes.Cut(lines.Bytes(), []byte{':'}); found {
			switch string(bytes.TrimSpace(key)) {
			case "physical id":
				currentCPUID = string(bytes.TrimSpace(value))
				if idx, found := slices.BinarySearch(cpuIDs, currentCPUID); !found {
					cpuIDs = slices.Insert(cpuIDs, idx, currentCPUID)
				}
			case "core id":
				coreID := currentCPUID + "/" + string(bytes.TrimSpace(value))
				if idx, found := slices.BinarySearch(coreIDs, coreID); !found {
					coreIDs = slices.Insert(coreIDs, idx, coreID)
				}
			}
		}
	}
	if lines.Err() != nil {
		return 0, 0
	}

	return uint(len(cpuIDs)), uint(len(coreIDs))
}
