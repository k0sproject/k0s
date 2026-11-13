// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestOSNameAndVersion(t *testing.T) {
	for _, test := range []struct {
		name                  string
		files                 map[string]string
		wantName, wantVersion string
	}{
		{
			name: "quoted values",
			files: map[string]string{"etc/os-release": `NAME="Some Linux"
VERSION_ID="12.34"
ID=some
PRETTY_NAME="Some Linux 12.34 (Made Up)"
`},
			wantName: "Some Linux 12.34 (Made Up)", wantVersion: "12.34",
		},
		{
			name: "unquoted values",
			files: map[string]string{"etc/os-release": `ID=some
VERSION_ID=12.34.56
PRETTY_NAME=Some
`},
			wantName: "Some", wantVersion: "12.34.56",
		},
		{
			name: "no version ID",
			files: map[string]string{"etc/os-release": `ID=rolling
PRETTY_NAME="Rolling Linux"
`},
			wantName: "Rolling Linux",
		},
		{
			name: "AlmaLinux reports the major version of the release file",
			files: map[string]string{
				"etc/os-release": `ID="almalinux"
VERSION_ID="12.34"
PRETTY_NAME="AlmaLinux 12.34 (Made Up)"
`,
				"etc/almalinux-release": "AlmaLinux release 56.78 (Made Up)\n",
			},
			wantName: "AlmaLinux 12.34 (Made Up)", wantVersion: "56",
		},
		{
			name: "AlmaLinux without a release file reports the major version ID",
			files: map[string]string{"etc/os-release": `ID="almalinux"
VERSION_ID="12.34"
PRETTY_NAME="AlmaLinux 12.34 (Made Up)"
`},
			wantName: "AlmaLinux 12.34 (Made Up)", wantVersion: "12",
		},
		{
			name: "Rocky Linux reports the major version of the release file",
			files: map[string]string{
				"etc/os-release": `ID="rocky"
VERSION_ID="12.34"
PRETTY_NAME="Rocky Linux 12.34 (Made Up)"
`,
				"etc/rocky-release": "Rocky Linux release 56.78 (Made Up)\n",
			},
			wantName: "Rocky Linux 12.34 (Made Up)", wantVersion: "56",
		},
		{
			name: "Rocky Linux with an unexpected release file reports the major version ID",
			files: map[string]string{
				"etc/os-release": `ID="rocky"
VERSION_ID="12.34"
PRETTY_NAME="Rocky Linux 12.34 (Made Up)"
`,
				"etc/rocky-release": "Something else entirely\n",
			},
			wantName: "Rocky Linux 12.34 (Made Up)", wantVersion: "12",
		},
		{
			name: "os-release file in /usr/lib only",
			files: map[string]string{"usr/lib/os-release": `ID=some
VERSION_ID=12.34
PRETTY_NAME="Some Linux"
`},
			wantName: "Some Linux", wantVersion: "12.34",
		},
		{
			name: "os-release file in /etc takes precedence over the one in /usr/lib",
			files: map[string]string{
				"etc/os-release": `ID=some
VERSION_ID=12.34
PRETTY_NAME="Some Linux"
`,
				"usr/lib/os-release": `ID=other
VERSION_ID=56.78
PRETTY_NAME="Other Linux"
`,
			},
			wantName: "Some Linux", wantVersion: "12.34",
		},
		{
			name: "no os-release file",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			name, version := osNameAndVersion(mapFS(test.files))

			assert.Equal(t, test.wantName, name, "Unexpected OS name")
			assert.Equal(t, test.wantVersion, version, "Unexpected OS version")
		})
	}
}

func TestCountCPUsAndCores(t *testing.T) {
	for _, test := range []struct {
		name                string
		files               map[string]string
		wantCPUs, wantCores uint
	}{
		{
			name: "two packages with two hyper-threaded cores each",
			files: map[string]string{"cpuinfo": "" +
				"processor\t: 0\nmodel name\t: Made Up CPU\nphysical id\t: 0\ncore id\t\t: 0\n\n" +
				"processor\t: 1\nmodel name\t: Made Up CPU\nphysical id\t: 0\ncore id\t\t: 1\n\n" +
				"processor\t: 2\nmodel name\t: Made Up CPU\nphysical id\t: 1\ncore id\t\t: 0\n\n" +
				"processor\t: 3\nmodel name\t: Made Up CPU\nphysical id\t: 1\ncore id\t\t: 1\n\n" +
				"processor\t: 4\nmodel name\t: Made Up CPU\nphysical id\t: 0\ncore id\t\t: 0\n\n" +
				"processor\t: 5\nmodel name\t: Made Up CPU\nphysical id\t: 0\ncore id\t\t: 1\n\n" +
				"processor\t: 6\nmodel name\t: Made Up CPU\nphysical id\t: 1\ncore id\t\t: 0\n\n" +
				"processor\t: 7\nmodel name\t: Made Up CPU\nphysical id\t: 1\ncore id\t\t: 1\n\n",
			},
			wantCPUs: 2, wantCores: 4,
		},
		{
			name: "no physical and core IDs",
			files: map[string]string{"cpuinfo": "" +
				"processor\t: 0\nCPU implementer\t: 0x41\nCPU part\t: 0xd03\n\n" +
				"processor\t: 1\nCPU implementer\t: 0x41\nCPU part\t: 0xd03\n\n",
			},
		},
		{
			name: "no cpuinfo file",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cpus, cores := countCPUsAndCores(mapFS(test.files))

			assert.Equal(t, test.wantCPUs, cpus, "Unexpected number of CPUs")
			assert.Equal(t, test.wantCores, cores, "Unexpected number of CPU cores")
		})
	}
}

func mapFS(files map[string]string) fstest.MapFS {
	fsys := make(fstest.MapFS, len(files))
	for path, content := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}
