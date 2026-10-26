// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	v1beta4 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	clusterinfophase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/clusterinfo"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/uploadconfig"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func caPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "kubernetes-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func byName(t *testing.T, objects []runtime.Object) map[string]runtime.Object {
	t.Helper()
	m := map[string]runtime.Object{}
	for _, o := range objects {
		meta, ok := o.(metav1.Object)
		require.True(t, ok)
		kind := o.GetObjectKind().GroupVersionKind().Kind
		m[kind+"/"+meta.GetNamespace()+"/"+meta.GetName()] = o
	}
	return m
}

func TestRender(t *testing.T) {
	ca := caPEM(t)
	objects, err := Render(&Input{
		ControlPlaneEndpoint: "10.0.0.1:6443", CACertPEM: ca,
		KubernetesVersion: "v1.36.4", ServiceCIDR: "10.96.0.0/12", PodCIDR: "10.244.0.0/16",
		ClusterDomain: "cluster.local", DNSAddress: "10.96.0.10",
	})
	require.NoError(t, err)
	objs := byName(t, objects)

	clusterInfo, ok := objs["ConfigMap/kube-public/cluster-info"].(*corev1.ConfigMap)
	require.True(t, ok)
	assert.Same(t, clusterInfo, objects[len(objects)-1], "cluster-info comes after everything joining nodes read")
	kubeconfig, err := clientcmd.Load([]byte(clusterInfo.Data["kubeconfig"]))
	require.NoError(t, err)
	require.Len(t, kubeconfig.Clusters, 1)
	assert.Equal(t, "https://10.0.0.1:6443", kubeconfig.Clusters[""].Server)
	assert.Equal(t, ca, kubeconfig.Clusters[""].CertificateAuthorityData)
	assert.Empty(t, kubeconfig.AuthInfos, "cluster-info is public")

	binding, ok := objs["RoleBinding/kube-public/"+clusterinfophase.BootstrapSignerClusterRoleName].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "system:anonymous"}}, binding.Subjects)

	clusterConfigMap, ok := objs["ConfigMap/kube-system/"+constants.KubeadmConfigConfigMap].(*corev1.ConfigMap)
	require.True(t, ok)
	var clusterConfiguration v1beta4.ClusterConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(clusterConfigMap.Data["ClusterConfiguration"]), &clusterConfiguration))
	assert.Equal(t, v1beta4.SchemeGroupVersion.String(), clusterConfiguration.APIVersion)
	assert.Equal(t, "v1.36.4", clusterConfiguration.KubernetesVersion)
	assert.Equal(t, "10.0.0.1:6443", clusterConfiguration.ControlPlaneEndpoint)
	assert.Equal(t, "10.96.0.0/12", clusterConfiguration.Networking.ServiceSubnet)
	assert.Equal(t, "10.244.0.0/16", clusterConfiguration.Networking.PodSubnet)

	kubeletConfigMap, ok := objs["ConfigMap/kube-system/kubelet-config"].(*corev1.ConfigMap)
	require.True(t, ok)
	assert.NotEmpty(t, kubeletConfigMap.Annotations[constants.ComponentConfigHashAnnotationKey], "signed for its readers")
	var kubeletConfig kubeletv1beta1.KubeletConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(kubeletConfigMap.Data["kubelet"]), &kubeletConfig))
	assert.Equal(t, []string{"10.96.0.10"}, kubeletConfig.ClusterDNS)
	assert.Equal(t, "cluster.local", kubeletConfig.ClusterDomain)
	assert.Equal(t, "/etc/kubernetes/pki/ca.crt", kubeletConfig.Authentication.X509.ClientCAFile)
	assert.True(t, kubeletConfig.ServerTLSBootstrap, "serving certificates from the cluster CA")

	for _, name := range []string{
		"Role/kube-system/" + uploadconfig.NodesKubeadmConfigClusterRoleName, "RoleBinding/kube-system/" + uploadconfig.NodesKubeadmConfigClusterRoleName,
		"Role/kube-system/" + constants.KubeletBaseConfigMapRole, "RoleBinding/kube-system/" + constants.KubeletBaseConfigMapRole,
		"ClusterRole//" + constants.GetNodesClusterRoleName, "ClusterRoleBinding//" + constants.GetNodesClusterRoleName,
	} {
		assert.Contains(t, objs, name)
	}
}

// renderInput returns a valid input with the given endpoint and CIDRs.
func renderInput(t *testing.T, endpoint, serviceCIDR, podCIDR, dnsAddress string) *Input {
	t.Helper()
	return &Input{
		ControlPlaneEndpoint: endpoint, CACertPEM: caPEM(t),
		KubernetesVersion: "v1.36.4", ServiceCIDR: serviceCIDR, PodCIDR: podCIDR,
		ClusterDomain: "cluster.local", DNSAddress: dnsAddress,
	}
}

func TestRender_IPv6AndDualStack(t *testing.T) {
	objects, err := Render(renderInput(t, "[fd00::1]:6443", "fd01::/108,10.96.0.0/12", "fd02::/48,10.244.0.0/16", "fd01::a"))
	require.NoError(t, err)
	objs := byName(t, objects)

	clusterInfo, ok := objs["ConfigMap/kube-public/cluster-info"].(*corev1.ConfigMap)
	require.True(t, ok)
	kubeconfig, err := clientcmd.Load([]byte(clusterInfo.Data["kubeconfig"]))
	require.NoError(t, err)
	assert.Equal(t, "https://[fd00::1]:6443", kubeconfig.Clusters[""].Server)

	clusterConfigMap, ok := objs["ConfigMap/kube-system/"+constants.KubeadmConfigConfigMap].(*corev1.ConfigMap)
	require.True(t, ok)
	var clusterConfiguration v1beta4.ClusterConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(clusterConfigMap.Data["ClusterConfiguration"]), &clusterConfiguration))
	assert.Equal(t, "[fd00::1]:6443", clusterConfiguration.ControlPlaneEndpoint)
	// Both families in the order of the primary one, as joining nodes expect them.
	assert.Equal(t, "fd01::/108,10.96.0.0/12", clusterConfiguration.Networking.ServiceSubnet)
	assert.Equal(t, "fd02::/48,10.244.0.0/16", clusterConfiguration.Networking.PodSubnet)

	kubeletConfigMap, ok := objs["ConfigMap/kube-system/kubelet-config"].(*corev1.ConfigMap)
	require.True(t, ok)
	var kubeletConfig kubeletv1beta1.KubeletConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(kubeletConfigMap.Data["kubelet"]), &kubeletConfig))
	assert.Equal(t, []string{"fd01::a"}, kubeletConfig.ClusterDNS)
}

func TestRender_KubeletConfiguration(t *testing.T) {
	objects, err := Render(renderInput(t, "10.0.0.1:6443", "10.96.0.0/12", "10.244.0.0/16", "10.96.0.10"))
	require.NoError(t, err)
	kubeletConfigMap, ok := byName(t, objects)["ConfigMap/kube-system/kubelet-config"].(*corev1.ConfigMap)
	require.True(t, ok)
	var kubeletConfig kubeletv1beta1.KubeletConfiguration
	require.NoError(t, yaml.UnmarshalStrict([]byte(kubeletConfigMap.Data["kubelet"]), &kubeletConfig))

	assert.Equal(t, "kubelet.config.k8s.io/v1beta1", kubeletConfig.APIVersion)
	assert.Equal(t, "KubeletConfiguration", kubeletConfig.Kind)
	// The kubelet API only serves authenticated and authorized requests, as with upstream defaults.
	assert.Equal(t, new(false), kubeletConfig.Authentication.Anonymous.Enabled)
	assert.Equal(t, new(true), kubeletConfig.Authentication.Webhook.Enabled)
	assert.Equal(t, kubeletv1beta1.KubeletAuthorizationModeWebhook, kubeletConfig.Authorization.Mode)
	assert.True(t, kubeletConfig.RotateCertificates)
	assert.Equal(t, "systemd", kubeletConfig.CgroupDriver)
	assert.Equal(t, "/etc/kubernetes/manifests", kubeletConfig.StaticPodPath)
	assert.Equal(t, "127.0.0.1", kubeletConfig.HealthzBindAddress)
	assert.Equal(t, ptr.To[int32](10248), kubeletConfig.HealthzPort)
}

func TestRender_BootstrapRBAC(t *testing.T) {
	objects, err := Render(renderInput(t, "10.0.0.1:6443", "10.96.0.0/12", "10.244.0.0/16", "10.96.0.10"))
	require.NoError(t, err)
	objs := byName(t, objects)

	bootstrappers := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: constants.NodeBootstrapTokenAuthGroup}
	nodes := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:nodes"}

	// Joining nodes read the configuration with their bootstrap token, and later as nodes.
	for _, name := range []string{uploadconfig.NodesKubeadmConfigClusterRoleName, constants.KubeletBaseConfigMapRole} {
		binding, ok := objs["RoleBinding/kube-system/"+name].(*rbacv1.RoleBinding)
		require.True(t, ok, name)
		assert.Equal(t, []rbacv1.Subject{bootstrappers, nodes}, binding.Subjects, name)
		assert.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}, binding.RoleRef, name)
	}
	for name, configMap := range map[string]string{
		uploadconfig.NodesKubeadmConfigClusterRoleName: constants.KubeadmConfigConfigMap,
		constants.KubeletBaseConfigMapRole:             constants.KubeletBaseConfigurationConfigMap,
	} {
		role, ok := objs["Role/kube-system/"+name].(*rbacv1.Role)
		require.True(t, ok, name)
		assert.Equal(t, []rbacv1.PolicyRule{{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{configMap},
		}}, role.Rules, name)
	}

	// Joining nodes check if a node with the same name exists.
	getNodes, ok := objs["ClusterRole//"+constants.GetNodesClusterRoleName].(*rbacv1.ClusterRole)
	require.True(t, ok)
	assert.Equal(t, []rbacv1.PolicyRule{{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"nodes"}}}, getNodes.Rules)
	getNodesBinding, ok := objs["ClusterRoleBinding//"+constants.GetNodesClusterRoleName].(*rbacv1.ClusterRoleBinding)
	require.True(t, ok)
	assert.Equal(t, []rbacv1.Subject{bootstrappers}, getNodesBinding.Subjects)

	// Nothing grants more than reading.
	for name, obj := range objs {
		var rules []rbacv1.PolicyRule
		switch role := obj.(type) {
		case *rbacv1.Role:
			rules = role.Rules
		case *rbacv1.ClusterRole:
			rules = role.Rules
		}
		for _, rule := range rules {
			assert.Equal(t, []string{"get"}, rule.Verbs, name)
		}
	}
}

func TestRender_AnonymousAccess(t *testing.T) {
	objects, err := Render(&Input{
		ControlPlaneEndpoint: "10.0.0.1:6443", CACertPEM: caPEM(t),
		KubernetesVersion: "v1.36.4", ServiceCIDR: "10.96.0.0/12", PodCIDR: "10.244.0.0/16",
		ClusterDomain: "cluster.local", DNSAddress: "10.96.0.10",
	})
	require.NoError(t, err)
	objs := byName(t, objects)

	// Only one binding grants anything to anonymous or unauthenticated requests.
	anonymous := func(subject rbacv1.Subject) bool {
		return subject.Name == "system:anonymous" || subject.Name == "system:unauthenticated"
	}
	var bindings []string
	for name, obj := range objs {
		var subjects []rbacv1.Subject
		switch binding := obj.(type) {
		case *rbacv1.RoleBinding:
			subjects = binding.Subjects
		case *rbacv1.ClusterRoleBinding:
			subjects = binding.Subjects
		}
		if slices.ContainsFunc(subjects, anonymous) {
			bindings = append(bindings, name)
		}
	}
	assert.Equal(t, []string{"RoleBinding/kube-public/" + clusterinfophase.BootstrapSignerClusterRoleName}, bindings)

	// It grants getting the cluster-info, and nothing else.
	binding, ok := objs["RoleBinding/kube-public/"+clusterinfophase.BootstrapSignerClusterRoleName].(*rbacv1.RoleBinding)
	require.True(t, ok)
	assert.Equal(t, []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "system:anonymous"}}, binding.Subjects)
	assert.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: clusterinfophase.BootstrapSignerClusterRoleName}, binding.RoleRef)
	role, ok := objs["Role/kube-public/"+clusterinfophase.BootstrapSignerClusterRoleName].(*rbacv1.Role)
	require.True(t, ok)
	assert.Equal(t, []rbacv1.PolicyRule{{
		Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{"cluster-info"},
	}}, role.Rules)
}
