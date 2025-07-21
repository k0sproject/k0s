// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
			require.NoErrorf(t, err, "expected an unsigned integer: %s", line)
			require.Equalf(t, kibiBytes*1024, ram, "/proc/meminfo differs: %s", line)
			return
		}
	}

	t.Error("No MemTotal found in /proc/meminfo.")
}
