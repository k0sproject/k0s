//go:build linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type cgroupV1 struct {
	controllers cgroupControllerProber
}

func (*cgroupV1) String() string {
	return "version 1"
}

func (g *cgroupV1) probeController(controllerName string) (cgroupControllerAvailable, error) {
	return g.controllers.probeController(g, controllerName)
}

func (g *cgroupV1) loadControllers(seen func(name, msg string)) error {
	// Get the available controllers from /proc/cgroups.
	// See https://www.man7.org/linux/man-pages/man7/cgroups.7.html#NOTES

	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return fmt.Errorf("failed to open /proc/cgroups: %w", err)
	}
	defer f.Close()

	var lineNo uint
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		lineNo++
		if err := lines.Err(); err != nil {
			return fmt.Errorf("failed to parse /proc/cgroups at line %d: %w ", lineNo, err)
		}
		text := lines.Text()
		if text[0] != '#' {
			parts := strings.Fields(text)
			if len(parts) >= 4 && parts[3] != "0" {
				seen(parts[0], "")
			}
		}
	}

	return nil
}
