// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package clusterinfo publishes the cluster-info ConfigMap for bootstrap token discovery.
// Along with it go the cluster and kubelet configuration that joining nodes read, and their RBAC.
package clusterinfo

import (
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	v1beta4 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/componentconfigs"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	clusterinfophase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/clusterinfo"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/uploadconfig"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// Input holds what's needed to render the objects that joining nodes read.
type Input struct {
	// The host and port of the API server, as seen from the workers.
	ControlPlaneEndpoint string

	KubernetesVersion string
	ServiceCIDR       string
	PodCIDR           string
	ClusterDomain     string
	DNSAddress        string

	CACertPEM []byte
}

// Render returns the cluster-info and the configuration that nodes joining via bootstrap tokens read.
// The node bootstrap bindings are part of the k0s system RBAC already.
func Render(in *Input) ([]runtime.Object, error) {
	clusterInfo, err := clientcmd.Write(clientcmdapi.Config{Clusters: map[string]*clientcmdapi.Cluster{"": {
		Server:                   "https://" + in.ControlPlaneEndpoint,
		CertificateAuthorityData: in.CACertPEM,
	}}})
	if err != nil {
		return nil, fmt.Errorf("failed to render cluster-info: %w", err)
	}

	clusterConfiguration, err := yaml.Marshal(&v1beta4.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta4.SchemeGroupVersion.String(),
			Kind:       "ClusterConfiguration",
		},
		ClusterName:          "k0s",
		KubernetesVersion:    in.KubernetesVersion,
		ControlPlaneEndpoint: in.ControlPlaneEndpoint,
		CertificatesDir:      v1beta4.DefaultCertificatesDir,
		Networking: v1beta4.Networking{
			ServiceSubnet: in.ServiceCIDR,
			PodSubnet:     in.PodCIDR,
			DNSDomain:     in.ClusterDomain,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render the cluster configuration: %w", err)
	}

	kubeletConfiguration, err := yaml.Marshal(kubeletConfiguration(in))
	if err != nil {
		return nil, fmt.Errorf("failed to render the kubelet configuration: %w", err)
	}
	kubeletConfigMap := configMap(metav1.NamespaceSystem, constants.KubeletBaseConfigurationConfigMap,
		constants.KubeletBaseConfigurationConfigMapKey, kubeletConfiguration)
	// Mark it as generated, as its readers expect.
	componentconfigs.SignConfigMap(kubeletConfigMap)

	bootstrappers := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: constants.NodeBootstrapTokenAuthGroup}
	nodes := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: constants.NodesGroup}

	// The applier creates the objects in order, so cluster-info comes last. Its presence means joining can
	// proceed in most cases.
	return []runtime.Object{
		configMap(metav1.NamespaceSystem, constants.KubeadmConfigConfigMap, constants.ClusterConfigurationConfigMapKey, clusterConfiguration),
		getConfigMapRole(metav1.NamespaceSystem, uploadconfig.NodesKubeadmConfigClusterRoleName, constants.KubeadmConfigConfigMap),
		roleBinding(metav1.NamespaceSystem, uploadconfig.NodesKubeadmConfigClusterRoleName, bootstrappers, nodes),

		kubeletConfigMap,
		getConfigMapRole(metav1.NamespaceSystem, constants.KubeletBaseConfigMapRole, constants.KubeletBaseConfigurationConfigMap),
		roleBinding(metav1.NamespaceSystem, constants.KubeletBaseConfigMapRole, bootstrappers, nodes),

		&rbacv1.ClusterRole{
			TypeMeta:   metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "ClusterRole"},
			ObjectMeta: metav1.ObjectMeta{Name: constants.GetNodesClusterRoleName},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"nodes"}}},
		},
		&rbacv1.ClusterRoleBinding{
			TypeMeta:   metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
			ObjectMeta: metav1.ObjectMeta{Name: constants.GetNodesClusterRoleName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: constants.GetNodesClusterRoleName},
			Subjects:   []rbacv1.Subject{bootstrappers},
		},

		getConfigMapRole(metav1.NamespacePublic, clusterinfophase.BootstrapSignerClusterRoleName, bootstrapapi.ConfigMapClusterInfo),
		roleBinding(metav1.NamespacePublic, clusterinfophase.BootstrapSignerClusterRoleName, rbacv1.Subject{Kind: rbacv1.UserKind, Name: user.Anonymous}),
		configMap(metav1.NamespacePublic, bootstrapapi.ConfigMapClusterInfo, bootstrapapi.KubeConfigKey, clusterInfo),
	}, nil
}

// kubeletConfiguration mirrors common upstream defaults. Joining nodes may patch it on their side.
func kubeletConfiguration(in *Input) *kubeletv1beta1.KubeletConfiguration {
	return &kubeletv1beta1.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubeletv1beta1.SchemeGroupVersion.String(),
			Kind:       "KubeletConfiguration",
		},
		Authentication: kubeletv1beta1.KubeletAuthentication{
			Anonymous: kubeletv1beta1.KubeletAnonymousAuthentication{Enabled: new(false)},
			Webhook:   kubeletv1beta1.KubeletWebhookAuthentication{Enabled: new(true)},
			X509:      kubeletv1beta1.KubeletX509Authentication{ClientCAFile: v1beta4.DefaultCertificatesDir + "/" + constants.CACertName},
		},
		Authorization: kubeletv1beta1.KubeletAuthorization{
			Mode: kubeletv1beta1.KubeletAuthorizationModeWebhook,
		},
		CgroupDriver:       constants.CgroupDriverSystemd,
		ClusterDNS:         []string{in.DNSAddress},
		ClusterDomain:      in.ClusterDomain,
		RotateCertificates: true,
		// The API server only trusts kubelet certificates of the cluster CA, which k0s signs on request.
		ServerTLSBootstrap: true,
		// The nodes run Linux, so the path uses forward slashes on any platform.
		StaticPodPath:      path.Join(constants.KubernetesDir, constants.ManifestsSubDirName),
		HealthzBindAddress: "127.0.0.1",
		HealthzPort:        ptr.To[int32](constants.KubeletHealthzPort),
	}
}

func configMap(namespace, name, key string, value []byte) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data:       map[string]string{key: string(value)},
	}
}

func getConfigMapRole(namespace, name, configMapName string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta:   metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "Role"},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Rules: []rbacv1.PolicyRule{{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{configMapName},
		}},
	}
}

// roleBinding binds the Role with the same name.
func roleBinding(namespace, name string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta:   metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "RoleBinding"},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name},
		Subjects:   subjects,
	}
}
