/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package constant

import (
	"os"
	"path/filepath"
	"runtime"
)

// WinDataDirDefault default data-dir for windows
// this one is defined here because it is used not only on windows worker but also during the control plane bootstrap
const WinDataDirDefault = "C:\\var\\lib\\k0s"

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
	// this relates to files like: admin.conf, kube-apiserver.yaml, certificate files, and more
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

	// User accounts for services

	// EtcdUser defines the user to use for running etcd process
	EtcdUser = "etcd"
	// KineUser defines the user to use for running kine process
	KineUser = "kube-apiserver" // apiserver needs to be able to read the kine unix socket
	// ApiserverUser defines the user to use for running k8s api-server process
	ApiserverUser = "kube-apiserver"
	// SchedulerUser defines the user to use for running k8s scheduler
	SchedulerUser = "kube-scheduler"
	// KonnectivityServerUser deinfes the user to use for konnectivity-server
	KonnectivityServerUser = "konnectivity-server"
	// KubernetesMajorMinorVersion defines the current embedded major.minor version info
	KubernetesMajorMinorVersion = "1.22"
	// DefaultPSP defines the system level default PSP to apply
	DefaultPSP = "00-k0s-privileged"
	// Image Constants
	KonnectivityImage                  = "k8s.gcr.io/kas-network-proxy/proxy-agent"
	KonnectivityImageVersion           = "v0.0.24"
	MetricsImage                       = "k8s.gcr.io/metrics-server/metrics-server"
	MetricsImageVersion                = "v0.5.0"
	KubeProxyImage                     = "k8s.gcr.io/kube-proxy"
	KubeProxyImageVersion              = "v1.22.2"
	CoreDNSImage                       = "k8s.gcr.io/coredns/coredns"
	CoreDNSImageVersion                = "v1.7.0"
	CalicoImage                        = "quay.io/calico/cni"
	CalicoComponentImagesVersion       = "v3.18.1"
	CalicoNodeImage                    = "quay.io/calico/node"
	KubeControllerImage                = "quay.io/calico/kube-controllers"
	KubeRouterCNIImage                 = "docker.io/cloudnativelabs/kube-router"
	KubeRouterCNIImageVersion          = "v1.3.1"
	KubeRouterCNIInstallerImage        = "quay.io/k0sproject/cni-node"
	KubeRouterCNIInstallerImageVersion = "0.1.0"

	// Controller component names
	APIConfigComponentName             = "api-config"
	ControlAPIComponentName            = "control-api"
	CoreDNSComponentname               = "coredns"
	CsrApproverComponentName           = "csr-approver"
	DefaultPspComponentName            = "default-psp"
	HelmComponentName                  = "helm"
	KonnectivityServerComponentName    = "konnectivity-server"
	KubeControllerManagerComponentName = "kube-controller-manager"
	KubeProxyComponentName             = "kube-proxy"
	KubeSchedulerComponentName         = "kube-scheduler"
	KubeletConfigComponentName         = "kubelet-config"
	MetricsServerComponentName         = "metrics-server"
	NetworkProviderComponentName       = "network-provider"
	SystemRbacComponentName            = "system-rbac"

	// ClusterConfigNamespace is the namespace where we expect to find the ClusterConfig CRs
	ClusterConfigNamespace  = "kube-system"
	ClusterConfigObjectName = "k0s"
)

// CfgVars is a struct that holds all the config variables required for K0s
type CfgVars struct {
	AdminKubeConfigPath        string // The cluster admin kubeconfig location
	BinDir                     string // location for all pki related binaries
	CertRootDir                string // CertRootDir defines the root location for all pki related artifacts
	WindowsCertRootDir         string // WindowsCertRootDir defines the root location for all pki related artifacts
	DataDir                    string // Data directory containing k0s state
	EtcdCertDir                string // EtcdCertDir contains etcd certificates
	EtcdDataDir                string // EtcdDataDir contains etcd state
	KineSocketPath             string // The unix socket path for kine
	KonnectivitySocketDir      string // location of konnectivity's socket path
	KubeletAuthConfigPath      string // KubeletAuthConfigPath defines the default kubelet auth config path
	KubeletBootstrapConfigPath string // KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletVolumePluginDir     string // location for kubelet plugins volume executables
	ManifestsDir               string // location for all stack manifests
	RunDir                     string // location of supervised pid files and sockets
	KonnectivityKubeConfigPath string // location for konnectivity kubeconfig
	OCIBundleDir               string // location for OCI bundles
	DefaultStorageType         string // Default backend storage

	// Helm config
	HelmHome             string
	HelmRepositoryCache  string
	HelmRepositoryConfig string
}

// GetConfig returns the pointer to a Config struct
func GetConfig(dataDir string) CfgVars {
	if dataDir == "" {
		switch runtime.GOOS {
		case "windows":
			dataDir = WinDataDirDefault
		default:
			dataDir = DataDirDefault
		}
	}

	// fetch absolute path for dataDir
	dataDir, _ = filepath.Abs(dataDir)

	var runDir string
	if os.Geteuid() == 0 {
		runDir = "/run/k0s"
	} else {
		runDir = formatPath(dataDir, "run")
	}
	certDir := formatPath(dataDir, "pki")
	winCertDir := WinDataDirDefault + "\\pki" // hacky but we need it to be windows style even on linux machine
	helmHome := formatPath(dataDir, "helmhome")

	return CfgVars{
		AdminKubeConfigPath:        formatPath(certDir, "admin.conf"),
		BinDir:                     formatPath(dataDir, "bin"),
		OCIBundleDir:               formatPath(dataDir, "images"),
		CertRootDir:                certDir,
		WindowsCertRootDir:         winCertDir,
		DataDir:                    dataDir,
		EtcdCertDir:                formatPath(certDir, "etcd"),
		EtcdDataDir:                formatPath(dataDir, "etcd"),
		KineSocketPath:             formatPath(runDir, KineSocket),
		KonnectivitySocketDir:      formatPath(runDir, "konnectivity-server"),
		KubeletAuthConfigPath:      formatPath(dataDir, "kubelet.conf"),
		KubeletBootstrapConfigPath: formatPath(dataDir, "kubelet-bootstrap.conf"),
		KubeletVolumePluginDir:     KubeletVolumePluginDir,
		ManifestsDir:               formatPath(dataDir, "manifests"),
		RunDir:                     runDir,
		KonnectivityKubeConfigPath: formatPath(certDir, "konnectivity.conf"),

		// Helm Config
		HelmHome:             helmHome,
		HelmRepositoryCache:  formatPath(helmHome, "cache"),
		HelmRepositoryConfig: formatPath(helmHome, "repositories.yaml"),
	}
}
