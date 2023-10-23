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

package probes

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTotalMemoryProber(t *testing.T) {
	underTest := newTotalMemoryProber()

	ram, err := underTest()
	require.NoError(t, err)

	procMeminfo, err := os.Open("/proc/meminfo")
	if os.IsNotExist(err) {
		t.Logf("Determined total RAM of %d bytes, /proc/meminfo not found, nothing to compare against...", ram)
		return
	}
	require.NoError(t, err)
	defer procMeminfo.Close()

	// https://github.com/torvalds/linux/blob/v4.9/fs/proc/meminfo.c#L68
	// https://github.com/torvalds/linux/blob/v2.6.28/fs/proc/meminfo.c#L56
	// https://github.com/torvalds/linux/blob/1da177e4c3f41524e886b7f1b8a0c1fc7321cac2/fs/proc/proc_misc.c#L149
	re := regexp.MustCompile(`^\s*MemTotal\s*:\s*(\d+)\s*kB\s*$`)

	lines := bufio.NewScanner(procMeminfo)
	lines.Split(bufio.ScanLines)
	for lines.Scan() {
		line := lines.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			kibiBytes, err := strconv.ParseUint(matches[1], 10, 64)
			require.NoError(t, err, "expected an unsigned integer: %s", line)
			require.Equal(t, kibiBytes*1024, ram, "/proc/meminfo differs: %s", line)
			return
		}
	}

	t.Error("No MemTotal found in /proc/meminfo.")
}
