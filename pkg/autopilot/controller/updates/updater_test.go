//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updates

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k0sproject/k0s/pkg/autopilot/channels"
	uc "github.com/k0sproject/k0s/pkg/autopilot/updater"
)

func TestToPlan_Sha256(t *testing.T) {
	u := &cronUpdater{}

	nextVersion := &uc.Update{
		Version: "v1.2.3+k0s.0",
		DownloadURLs: []channels.DownloadURL{
			{Arch: "amd64", OS: "linux", K0S: "http://example.com/k0s", K0SSha256: "deadbeef"},
			{Arch: "arm64", OS: "linux", K0S: "http://example.com/k0s-arm64"},
		},
	}

	plan := u.toPlan(nextVersion)
	platforms := plan.Spec.Commands[0].K0sUpdate.Platforms

	assert.Equal(t, "deadbeef", platforms["linux-amd64"].Sha256)
	assert.Empty(t, platforms["linux-arm64"].Sha256)
}
