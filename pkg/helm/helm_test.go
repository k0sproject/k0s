// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommands_locateChart_OCIRequiresFixedVersion(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	underTest := &Commands{
		repoFile:     tempDir + "/repositories.yaml",
		helmCacheDir: tempDir + "/cache",
	}

	for _, version := range []string{"", "latest"} {
		_, err := underTest.locateChart("oci://ghcr.io/k0sproject/charts/test", version, nil)
		require.ErrorContains(t, err, "OCI charts require a fixed SemVer version")
	}
}
