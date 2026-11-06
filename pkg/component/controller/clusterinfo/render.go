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

// Render returns the cluster-info, and the configuration and RBAC that nodes joining via bootstrap tokens read.
// The bindings for their certificate signing requests are part of the k0s system RBAC already.
func Render(in *Input) ([]runtime.Object, error) {
	// cluster-info content, lets joining nodes discover the API server and its CA before they have credentials.
	clusterInfo, err := clientcmd.Write(clientcmdapi.Config{Clusters: map[string]*clientcmdapi.Cluster{"": {
		Server:                   "https://" + in.ControlPlaneEndpoint,
		CertificateAuthorityData: in.CACertPEM,
	}}})
	if err != nil {
		return nil, fmt.Errorf("failed to render cluster-info: %w", err)
	}

	// kubeadm-config content, the cluster configuration that nodes read when joining, and later with their own credentials.
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

	// kubelet-config content, the kubelet configuration that nodes write to disk when joining.
	kubeletConfiguration, err := yaml.Marshal(&kubeletv1beta1.KubeletConfiguration{
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
		// The API server only trusts kubelet certificates of the cluster CA, so kubelets request them and k0s approves.
		ServerTLSBootstrap: true,
		// The nodes run Linux, so the path uses forward slashes on any platform.
		StaticPodPath:      path.Join(constants.KubernetesDir, constants.ManifestsSubDirName),
		HealthzBindAddress: "127.0.0.1",
		HealthzPort:        ptr.To[int32](constants.KubeletHealthzPort),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render the kubelet configuration: %w", err)
	}

	// Joining nodes authenticate with bootstrap tokens of this group. Joined nodes authenticate as nodes.
	bootstrappers := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: constants.NodeBootstrapTokenAuthGroup}
	nodes := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: constants.NodesGroup}

	// kubelet-config ConfigMap with the kubelet configuration.
	kubeletConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: constants.KubeletBaseConfigurationConfigMap},
		Data:       map[string]string{constants.KubeletBaseConfigurationConfigMapKey: string(kubeletConfiguration)},
	}
	// Mark it as generated, so that joining nodes don't treat it as supplied by the user.
	componentconfigs.SignConfigMap(kubeletConfigMap)

	// The stack is applied in order, so cluster-info comes last, after what joining nodes read.
	return []runtime.Object{
		// kubeadm-config ConfigMap with the cluster configuration.
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: constants.KubeadmConfigConfigMap},
			Data:       map[string]string{constants.ClusterConfigurationConfigMapKey: string(clusterConfiguration)},
		},
		// kubeadm-config Role, allows getting the kubeadm-config ConfigMap only.
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: uploadconfig.NodesKubeadmConfigClusterRoleName},
			Rules: []rbacv1.PolicyRule{{
				Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{constants.KubeadmConfigConfigMap},
			}},
		},
		// kubeadm-config RoleBinding, grants the Role to joining and joined nodes.
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: uploadconfig.NodesKubeadmConfigClusterRoleName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: uploadconfig.NodesKubeadmConfigClusterRoleName},
			Subjects:   []rbacv1.Subject{bootstrappers, nodes},
		},

		kubeletConfigMap,
		// kubelet-config Role, allows getting the kubelet-config ConfigMap only.
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: constants.KubeletBaseConfigMapRole},
			Rules: []rbacv1.PolicyRule{{
				Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{constants.KubeletBaseConfigurationConfigMap},
			}},
		},
		// kubelet-config RoleBinding, grants the Role to joining and joined nodes.
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: constants.KubeletBaseConfigMapRole},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: constants.KubeletBaseConfigMapRole},
			Subjects:   []rbacv1.Subject{bootstrappers, nodes},
		},

		// get-nodes ClusterRole, lets joining nodes check that no ready node with their name exists yet.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: constants.GetNodesClusterRoleName},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"nodes"}}},
		},
		// get-nodes ClusterRoleBinding, grants the ClusterRole to joining nodes.
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: constants.GetNodesClusterRoleName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: constants.GetNodesClusterRoleName},
			Subjects:   []rbacv1.Subject{bootstrappers},
		},

		// cluster-info Role, allows getting the cluster-info ConfigMap only.
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespacePublic, Name: clusterinfophase.BootstrapSignerClusterRoleName},
			Rules: []rbacv1.PolicyRule{{
				Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{bootstrapapi.ConfigMapClusterInfo},
			}},
		},
		// cluster-info RoleBinding, grants the Role to anonymous users. The authentication configuration of the
		// API server lets them through for that path only.
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespacePublic, Name: clusterinfophase.BootstrapSignerClusterRoleName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: clusterinfophase.BootstrapSignerClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: user.Anonymous}},
		},
		// cluster-info ConfigMap with the API server endpoint and CA.
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespacePublic, Name: bootstrapapi.ConfigMapClusterInfo},
			Data:       map[string]string{bootstrapapi.KubeConfigKey: string(clusterInfo)},
		},
	}, nil
}
