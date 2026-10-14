// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package nllb

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// APIServerURL is the local URL host-network pods use when node-local load
// balancing is enabled. A disabled config returns a nil URL.
//
// FIXME: The localhost address is not exactly on par with the way it's
// implemented on the worker side, i.e. there's no fallback if localhost
// doesn't resolve to a loopback address. But this would require some
// shenanigans to pull in node-specific values here. A possible solution
// would be to convert kube-proxy to a static Pod as well.
func APIServerURL(nllb *v1beta1.NodeLocalLoadBalancing) (*url.URL, error) {
	if !nllb.IsEnabled() {
		return nil, nil
	}

	var port int32
	switch nllb.Type {
	case v1beta1.NllbTypeEnvoyProxy:
		if nllb.EnvoyProxy == nil {
			return nil, fmt.Errorf("envoy proxy node-local load balancing config is missing")
		}
		port = nllb.EnvoyProxy.APIServerBindPort
	case v1beta1.NllbTypeTraefik:
		if nllb.Traefik == nil {
			return nil, fmt.Errorf("traefik node-local load balancing config is missing")
		}
		port = nllb.Traefik.APIServerBindPort
	default:
		return nil, fmt.Errorf("unsupported node-local load balancer type %q", nllb.Type)
	}

	return &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort("localhost", strconv.Itoa(int(port))),
	}, nil
}
