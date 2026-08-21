// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"strings"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSANList(t *testing.T) {
	spec := v1beta1.DefaultClusterSpec()
	spec.API.SANs = []string{"some.example.com"}
	underTest := Certificates{ClusterSpec: spec}

	sans, err := underTest.generateSANList(t.Context())
	require.NoError(t, err)

	assert.Contains(t, sans, "kubernetes.default.svc."+spec.Network.ClusterDomain)

	// Trailing dots make dNSName SANs malformed (RFC 5280 / RFC 1034), and
	// strict x509 parsers, such as Go in FIPS-140-3 mode, reject them.
	for _, san := range sans {
		assert.False(t, strings.HasSuffix(san, "."), "SAN must not end in a dot: %q", san)
	}
}
