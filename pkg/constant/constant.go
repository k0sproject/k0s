// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"crypto/tls"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Network providers
const (
	CNIProviderCalico     = "calico"
	CNIProviderKubeRouter = "kuberouter"
)

const (

	// DataDirMode is the expected directory permissions for DataDirDefault
	DataDirMode = 0755
	// EtcdDataDirMode is the expected directory permissions for EtcdDataDir. see https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	EtcdDataDirMode = 0700
	// CertRootDirMode is the expected directory permissions for CertRootDir.
	CertRootDirMode = 0751
	// EtcdCertDirMode is the expected directory permissions for EtcdCertDir
	EtcdCertDirMode = 0711
	// CertMode is the expected permissions for certificates. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.20/
	CertMode = 0644
	// CertSecureMode is the expected file permissions for secure files. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.13/
	// this relates to files like: kube-apiserver.yaml, certificate files, and more
	CertSecureMode = 0640
	// BinDirMode is the expected directory permissions for BinDir
	BinDirMode = 0755
	// RunDirMode is the expected permissions of RunDir
	RunDirMode = 0755
	// PidFileMode is the expected file permissions for pid files
	PidFileMode = 0644
	// ManifestsDirMode is the expected directory permissions for ManifestsDir
	ManifestsDirMode = 0755
	// KineDBDirMode is the expected directory permissions for the Kine DB
	KineDBDirMode = 0750
	// keepalived is the expected directory permissions for the Keepalived directory
	KeepalivedDirMode = 0600
	// OwnerOnlyMode is the expected file permissions for owner-only access files.
	// this relates to files like: admin.conf, kubelet config.yaml
	OwnerOnlyMode = 0600

	/* User accounts for services */

	// EtcdUser defines the user to use for running etcd process
	EtcdUser = "etcd"
	// KineUser defines the user to use for running kine process
	KineUser = "kube-apiserver" // apiserver needs to be able to read the kine unix socket
	// ApiserverUser defines the user to use for running k8s api-server process
	ApiserverUser = "kube-apiserver"
	// SchedulerUser defines the user to use for running k8s scheduler
	SchedulerUser = "kube-scheduler"
	// KonnectivityServerUser defines the user to use for konnectivity-server
	KonnectivityServerUser = "konnectivity-server"
	// KeepalivedUser defines the user to use for running keepalived
	KeepalivedUser = "keepalived"

	// KubernetesMajorMinorVersion defines the current embedded major.minor version info
	KubernetesMajorMinorVersion = "1.35"
	// Indicates if k0s is using a Kubernetes pre-release or a GA version.
	KubernetesPreRelease = false

	/* Image Constants */

	KonnectivityImage                     = "quay.io/k0sproject/apiserver-network-proxy-agent"
	KonnectivityImageVersion              = "v0.34.0-1"
	PushGatewayImage                      = "quay.io/k0sproject/pushgateway-ttl"
	PushGatewayImageVersion               = "1.4.0-k0s.0"
	MetricsImage                          = "quay.io/k0sproject/metrics-server"
	MetricsImageVersion                   = "v0.8.1-0"
	KubePauseContainerImage               = "quay.io/k0sproject/pause"
	KubePauseContainerImageVersion        = "3.10.1"
	KubePauseWindowsContainerImage        = "registry.k8s.io/pause"
	KubePauseWindowsContainerImageVersion = "3.10.1"
	KubeProxyImage                        = "quay.io/k0sproject/kube-proxy"
	KubeProxyImageVersion                 = "v1.35.1"
	KubeProxyWindowsImage                 = "docker.io/sigwindowstools/kube-proxy"
	KubeProxyWindowsImageVersion          = "v1.35.1-calico-hostprocess"
	CoreDNSImage                          = "quay.io/k0sproject/coredns"
	CoreDNSImageVersion                   = "1.14.1"
	EnvoyProxyImage                       = "quay.io/k0sproject/envoy-distroless"
	EnvoyProxyImageVersion                = "v1.37.0"
	CalicoCNIImage                        = "quay.io/k0sproject/calico-cni"
	CalicoComponentImagesVersion          = "v3.31.3-0"
	CalicoCNIWindowsImage                 = "docker.io/calico/cni-windows"
	CalicoCNIWindowsImageVersion          = "v3.31.3"
	CalicoNodeImage                       = "quay.io/k0sproject/calico-node"
	CalicoNodeWindowsImage                = "docker.io/calico/node-windows"
	CalicoNodeWindowsImageVersion         = "v3.31.3"
	KubeControllerImage                   = "quay.io/k0sproject/calico-kube-controllers"
	KubeRouterCNIImage                    = "quay.io/k0sproject/kube-router"
	KubeRouterCNIImageVersion             = "v2.6.3-iptables1.8.11-0"
	KubeRouterCNIInstallerImage           = "quay.io/k0sproject/cni-node"
	KubeRouterCNIInstallerImageVersion    = "1.8.0-k0s.0"

	/* Controller component names */

	APIEndpointReconcilerComponentName = "endpoint-reconciler"
	ApplierManagerComponentName        = "applier-manager"
	ControlAPIComponentName            = "control-api"
	CoreDNSComponentname               = "coredns"
	CsrApproverComponentName           = "csr-approver"
	HelmComponentName                  = "helm"
	IptablesBinariesComponentName      = "iptables-binaries"
	KonnectivityServerComponentName    = "konnectivity-server"
	KubeControllerManagerComponentName = "kube-controller-manager"
	KubeProxyComponentName             = "kube-proxy"
	KubeSchedulerComponentName         = "kube-scheduler"
	WorkerConfigComponentName          = "worker-config"
	MetricsServerComponentName         = "metrics-server"
	NetworkProviderComponentName       = "network-provider"
	SystemRBACComponentName            = "system-rbac"
	NodeRoleComponentName              = "node-role"
	WindowsNodeComponentName           = "windows-node"
	AutopilotComponentName             = "autopilot"
	UpdateProberComponentName          = "update-prober"

	// ClusterConfigNamespace is the namespace where we expect to find the ClusterConfig CRs
	ClusterConfigNamespace  = metav1.NamespaceSystem
	ClusterConfigObjectName = "k0s"

	K0SNodeRoleLabel = "node.k0sproject.io/role"
)

// The list of allowed TLS v1.2 cipher suites. Those should be used for k0s
// itself and all embedded components. Note that TLS v1.3 ciphers are currently
// not configurable in Go.
//
// https://ssl-config.mozilla.org/#server=go&config=intermediate
var AllowedTLS12CipherSuiteIDs = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

// A comma-separated string version of [AllowedTLS12CipherSuiteIDs], suitable to
// be used as CLI arg for binaries.
func AllowedTLS12CipherSuiteNames() string {
	var cipherSuites strings.Builder
	for i, cipherSuite := range AllowedTLS12CipherSuiteIDs {
		if i > 0 {
			cipherSuites.WriteRune(',')
		}
		cipherSuites.WriteString(tls.CipherSuiteName(cipherSuite))
	}
	return cipherSuites.String()
}
