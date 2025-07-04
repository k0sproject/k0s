//go:build !linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

func (a *assertDiskSpace) Probe(reporter Reporter) error {
	return reporter.Warn(a.desc(), probeUnsupported("Disk space detection unsupported on this platform"), "")
}
