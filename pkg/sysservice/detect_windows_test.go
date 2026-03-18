//go:build windows

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import "testing"

func TestDetect_Windows(t *testing.T) {
	kind, err := Detect("")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if kind != "windows" {
		t.Fatalf("expected windows, got %q", kind)
	}
}
