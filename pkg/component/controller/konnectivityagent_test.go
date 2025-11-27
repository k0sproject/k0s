// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/cli-runtime/pkg/resource"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKonnectivityAgent_ProxyServerHostPort(t *testing.T) {
	apiServerHosts := []struct {
		name, host string
	}{
		{"ipv4", "10.0.0.1"},
		{"ipv6", "::1"},
		{"host", "api.example.com"},
	}

	extKonnectivityAddresses := []struct {
		name, address, expectedHost, expectedPort string
	}{
		{"empty", "", "", ""},
		{"ipv4", "10.0.0.7", "10.0.0.7", ""},
		{"ipv6", "::7", "::7", ""},
		{"host", "konnectivity.example.com", "konnectivity.example.com", ""},
		{"ipv4_ExtPort", "10.0.0.7:5678", "10.0.0.7", "5678"},
		{"ipv6_ExtPort", "[::7]:5678", "::7", "5678"},
		{"host_ExtPort", "k8s.example.com:5678", "k8s.example.com", "5678"},
	}

	for _, apiServerHost := range apiServerHosts {
		for _, extKonnectivityAddress := range extKonnectivityAddresses {
			t.Run(apiServerHost.name+"_"+extKonnectivityAddress.name, func(t *testing.T) {
				expectedHost := cmp.Or(extKonnectivityAddress.expectedHost, apiServerHost.host)
				expectedPort := cmp.Or(extKonnectivityAddress.expectedPort, "9876")

				k0sVars, err := config.NewCfgVars(nil, t.TempDir())
				require.NoError(t, err)

				underTest := KonnectivityAgent{
					K0sVars:       k0sVars,
					APIServerHost: cmp.Or(extKonnectivityAddress.address, apiServerHost.host),
					EventEmitter:  prober.NewEventEmitter(),
				}

				require.NoError(t, underTest.writeKonnectivityAgent(&k0sv1beta1.ClusterConfig{
					Spec: &k0sv1beta1.ClusterSpec{
						Images: k0sv1beta1.DefaultClusterImages(),
						Konnectivity: &k0sv1beta1.KonnectivitySpec{
							AgentPort: 9876,
						},
					},
				}, 1))

				daemonSet := loadDaemonSet(t, k0sVars.ManifestsDir)
				containers := daemonSet.Spec.Template.Spec.Containers
				require.Len(t, containers, 1)
				args := containers[0].Args
				require.Len(t, args, 7)
				assert.Equal(t, "--proxy-server-host="+expectedHost, args[2])
				assert.Equal(t, "--proxy-server-port="+expectedPort, args[3])
			})
		}
	}
}

func loadDaemonSet(t *testing.T, manifestsDir string) *appsv1.DaemonSet {
	objects, err := resource.NewLocalBuilder().
		WithScheme(kubernetesscheme.Scheme,
			corev1.SchemeGroupVersion,
			appsv1.SchemeGroupVersion,
			rbacv1.SchemeGroupVersion,
		).
		Path(true, manifestsDir).
		Flatten().
		Do().
		Infos()
	require.NoError(t, err)

	require.Len(t, objects, 3)
	daemonSet, ok := objects[2].Object.(*appsv1.DaemonSet)
	require.Truef(t, ok, "unexpected type: %T", objects[2].Object)
	return daemonSet
}
