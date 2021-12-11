// Copyright © 2016 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// OS information.
type OS struct {
	Name         string `json:"name,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	Version      string `json:"version,omitempty"`
	Release      string `json:"release,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

const (
	osReleaseFile = "/etc/os-release"

	centOS6Template = `NAME="CentOS Linux"
VERSION="6 %s"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="6"
PRETTY_NAME="CentOS Linux 6 %s"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:6"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"`

	redhat6Template = `NAME="Red Hat Enterprise Linux Server"
VERSION="%s %s"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="%s"
PRETTY_NAME="Red Hat Enterprise Linux"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:redhat:enterprise_linux:%s:GA:server"
HOME_URL="https://www.redhat.com/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"`
)

var (
	rePrettyName = regexp.MustCompile(`^PRETTY_NAME=(.*)$`)
	reID         = regexp.MustCompile(`^ID=(.*)$`)
	reVersionID  = regexp.MustCompile(`^VERSION_ID=(.*)$`)
	reUbuntu     = regexp.MustCompile(`[\( ]([\d\.]+)`)
	reCentOS     = regexp.MustCompile(`^CentOS( Linux)? release ([\d\.]+) `)
	reCentOS6    = regexp.MustCompile(`^CentOS release 6\.\d+ (.*)`)
	reRedhat     = regexp.MustCompile(`[\( ]([\d\.]+)`)
	reRedhat6    = regexp.MustCompile(`^Red Hat Enterprise Linux Server release (.*) (.*)`)
)

func genOSRelease() {
	// CentOS 6.x
	if release := slurpFile("/etc/centos-release"); release != "" {
		if m := reCentOS6.FindStringSubmatch(release); m != nil {
			spewFile(osReleaseFile, fmt.Sprintf(centOS6Template, m[1], m[1]), 0666)
			return
		}
	}

	// RHEL 6.x
	if release := slurpFile("/etc/redhat-release"); release != "" {
		if m := reRedhat6.FindStringSubmatch(release); m != nil {
			version := "6"
			code_name := "()"
			switch l := len(m); l {
			case 3:
				code_name = m[2]
				fallthrough
			case 2:
				version = m[1]
			}
			spewFile(osReleaseFile, fmt.Sprintf(redhat6Template, version, code_name, version, version), 0666)
			return
		}
	}
}

func (si *SysInfo) getOSInfo() {
	// This seems to be the best and most portable way to detect OS architecture (NOT kernel!)
	if _, err := os.Stat("/lib64/ld-linux-x86-64.so.2"); err == nil {
		si.OS.Architecture = "amd64"
	} else if _, err := os.Stat("/lib/ld-linux.so.2"); err == nil {
		si.OS.Architecture = "i386"
	}

	if _, err := os.Stat(osReleaseFile); os.IsNotExist(err) {
		genOSRelease()
	}

	f, err := os.Open(osReleaseFile)
	if err != nil {
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if m := rePrettyName.FindStringSubmatch(s.Text()); m != nil {
			si.OS.Name = strings.Trim(m[1], `"`)
		} else if m := reID.FindStringSubmatch(s.Text()); m != nil {
			si.OS.Vendor = strings.Trim(m[1], `"`)
		} else if m := reVersionID.FindStringSubmatch(s.Text()); m != nil {
			si.OS.Version = strings.Trim(m[1], `"`)
		}
	}

	switch si.OS.Vendor {
	case "debian":
		si.OS.Release = slurpFile("/etc/debian_version")
	case "ubuntu":
		if m := reUbuntu.FindStringSubmatch(si.OS.Name); m != nil {
			si.OS.Release = m[1]
		}
	case "centos":
		if release := slurpFile("/etc/centos-release"); release != "" {
			if m := reCentOS.FindStringSubmatch(release); m != nil {
				si.OS.Release = m[2]
			}
		}
	case "rhel":
		if release := slurpFile("/etc/redhat-release"); release != "" {
			if m := reRedhat.FindStringSubmatch(release); m != nil {
				si.OS.Release = m[1]
			}
		}
		if len(si.OS.Release) == 0 {
			if m := reRedhat.FindStringSubmatch(si.OS.Name); m != nil {
				si.OS.Release = m[1]
			}
		}
	}
}
