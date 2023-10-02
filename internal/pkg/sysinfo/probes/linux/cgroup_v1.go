//go:build linux

/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		lineNo = lineNo + 1
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
