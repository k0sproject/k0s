//go:build !linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

func (a *assertFileSystem) Probe(reporter Reporter) error {
	return reporter.Warn(a.desc(), probeUnsupported("Filesystem detection unsupported on this platform"), "")
}
