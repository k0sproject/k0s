// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	v1beta4 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/componentconfigs"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	objects, err := Render(&Input{
		ControlPlaneEndpoint: "control-plane.example.com:6443",
		KubernetesVersion:    "v1.99.0",
		ServiceCIDR:          "10.99.0.0/16",
		PodCIDR:              "10.98.0.0/16",
		ClusterDomain:        "example.local",
		DNSAddress:           "10.99.0.10",
		CACertPEM:            []byte("made-up CA"),
	})
	require.NoError(t, err)

	configMaps := map[string]*corev1.ConfigMap{}
	for _, obj := range objects {
		if cm, ok := obj.(*corev1.ConfigMap); ok {
			configMaps[cm.Namespace+"/"+cm.Name] = cm
		}
	}

	clusterInfo := configMaps["kube-public/cluster-info"]
	require.NotNil(t, clusterInfo, "the cluster-info should be rendered")
	assert.Same(t, clusterInfo, objects[len(objects)-1], "the cluster-info should come last")
	kubeconfig, err := clientcmd.Load([]byte(clusterInfo.Data["kubeconfig"]))
	require.NoError(t, err)
	require.Contains(t, kubeconfig.Clusters, "", "the cluster-info should hold a cluster without a name")
	assert.Equal(t, "https://control-plane.example.com:6443", kubeconfig.Clusters[""].Server, "the server should be the endpoint")
	assert.Equal(t, []byte("made-up CA"), kubeconfig.Clusters[""].CertificateAuthorityData, "the CA should be included")
	assert.Empty(t, kubeconfig.AuthInfos, "the cluster-info should hold no credentials")

	clusterConfigMap := configMaps["kube-system/kubeadm-config"]
	require.NotNil(t, clusterConfigMap, "the cluster configuration should be rendered")
	var clusterConfig v1beta4.ClusterConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(clusterConfigMap.Data["ClusterConfiguration"]), &clusterConfig))
	assert.Equal(t, "v1.99.0", clusterConfig.KubernetesVersion, "the Kubernetes version should be passed on")
	assert.Equal(t, "control-plane.example.com:6443", clusterConfig.ControlPlaneEndpoint, "the endpoint should be passed on")
	assert.Equal(t, v1beta4.Networking{ServiceSubnet: "10.99.0.0/16", PodSubnet: "10.98.0.0/16", DNSDomain: "example.local"},
		clusterConfig.Networking, "the networking should be passed on")

	kubeletConfigMap := configMaps["kube-system/kubelet-config"]
	require.NotNil(t, kubeletConfigMap, "the kubelet configuration should be rendered")
	assert.True(t, componentconfigs.VerifyConfigMapSignature(kubeletConfigMap), "the kubelet configuration should be signed")
	var kubeletConfig kubeletv1beta1.KubeletConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(kubeletConfigMap.Data["kubelet"]), &kubeletConfig))
	assert.Equal(t, []string{"10.99.0.10"}, kubeletConfig.ClusterDNS, "the DNS address should be passed on")
	assert.Equal(t, "example.local", kubeletConfig.ClusterDomain, "the cluster domain should be passed on")
	assert.True(t, kubeletConfig.ServerTLSBootstrap, "kubelets should request serving certificates")
}
