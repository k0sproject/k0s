// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package nllb

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/require"
)

func TestAPIServerURL(t *testing.T) {
	url, err := APIServerURL(nil)
	require.NoError(t, err)
	require.Nil(t, url)

	url, err = APIServerURL(&v1beta1.NodeLocalLoadBalancing{Enabled: false})
	require.NoError(t, err)
	require.Nil(t, url)

	url, err = APIServerURL(&v1beta1.NodeLocalLoadBalancing{
		Enabled:    true,
		Type:       v1beta1.NllbTypeEnvoyProxy,
		EnvoyProxy: &v1beta1.EnvoyProxy{APIServerBindPort: 7443},
	})
	require.NoError(t, err)
	require.Equal(t, "https://localhost:7443", url.String())

	url, err = APIServerURL(&v1beta1.NodeLocalLoadBalancing{
		Enabled: true,
		Type:    v1beta1.NllbTypeTraefik,
		Traefik: &v1beta1.Traefik{APIServerBindPort: 7444},
	})
	require.NoError(t, err)
	require.Equal(t, "https://localhost:7444", url.String())

	_, err = APIServerURL(&v1beta1.NodeLocalLoadBalancing{
		Enabled: true,
		Type:    v1beta1.NllbTypeEnvoyProxy,
	})
	require.Error(t, err)

	_, err = APIServerURL(&v1beta1.NodeLocalLoadBalancing{
		Enabled: true,
		Type:    "nope",
	})
	require.Error(t, err)
}
