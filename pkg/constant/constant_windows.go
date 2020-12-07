// +build windows
/*
Copyright 2020 Mirantis, Inc.

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

const (
	// DataDirDefault folder contains all k0s state
	DataDirDefault = "C:\\var\\lib\\k0s"
	// DataDirMode is the expected directory permissions for DataDirDefault
	DataDirMode = 0755
	// EtcdDataDir contains etcd state
	EtcdDataDir = "C:\\var\\lib\\k0s\\etcd"
	// EtcdDataDirMode is the expected directory permissions for EtcdDataDir. see https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	EtcdDataDirMode = 0700
	// CertRootDir defines the root location for all pki related artifacts
	CertRootDir = "C:\\var\\lib\\k0s\\pki"
	// CertRootDirMode is the expected directory permissions for CertRootDir.
	CertRootDirMode = 0751
	//EtcdCertDir contains etcd certificates
	EtcdCertDir = "C:\\var\\lib\\k0s\\pki/etcd"
	// EtcdCertDirMode is the expected directory permissions for EtcdCertDir
	EtcdCertDirMode = 0711
	// CertMode is the expected permissions for certificates. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.20/
	CertMode = 0644
	// CertSecureMode is the expected file permissions for secure files. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.13/
	// this relates to files like: admin.conf, kube-apiserver.yaml, certificate files, and more
	CertSecureMode = 0640
	// BinDir defines the location for all pki related binaries
	BinDir = "C:\\var\\lib\\k0s\\bin"
	// BinDirMode is the expected directory permissions for BinDir
	BinDirMode = 0755
	// RunDir defines the location of supervised pid files and sockets
	RunDir = "C:\\run\\k0s"
	// RunDirMode is the expected permissions of RunDir
	RunDirMode = 0755
	// PidFileMode is the expected file permissions for pid files
	PidFileMode = 0644
	// ManifestsDir defines the location for all stack manifests
	ManifestsDir = "C:\\var\\lib\\k0s\\manifests"
	// ManifestsDirMode is the expected directory permissions for ManifestsDir
	ManifestsDirMode = 0644

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "C:\\var\\lib\\k0s\\kubelet-bootstrap.conf"
	// KubeletAuthConfigPath defines the default kubelet auth config path
	KubeletAuthConfigPath = "C:\\var\\lib\\k0s\\kubelet.conf"
	// KubeletVolumePluginDir defines the location for kubelet plugins volume executables
	KubeletVolumePluginDir = "C:\\usr\\libexec\\k0s\\kubelet-plugins\\volume\\exec"
	// KubeletVolumePlugindDirMode is the expected directory permissions for KubeleteVolumePluginDir
	KubeletVolumePluginDirMode = 0700

	// AdminKubeconfigConfigPath defines the cluster admin kubeconfig location
	AdminKubeconfigConfigPath = "C:\\var\\lib\\k0s\\pki\\admin.conf"

	// Group defines group name for shared directories
	Group = "k0s"

	// User accounts for services

	// EtcdUser defines the user to use for running etcd process
	EtcdUser = "etcd"
	// KineUser defines the user to use for running kine process
	KineUser = "kine"
	// ApiserverUser defines the user to use for running k8s api-server process
	ApiserverUser = "kube-apiserver"
	// ControllerManagerUser defines the user to use for running k8s controller manager process
	ControllerManagerUser = "kube-controller-manager"
	// SchedulerUser defines the user to use for running k8s scheduler
	SchedulerUser = "kube-scheduler"
	// KonnectivityServerUser deinfes the user to use for konnectivity-server
	KonnectivityServerUser = "konnectivity-server"

	// KubernetesMajorMinorVersion defines the current embedded major.minor version info
	KubernetesMajorMinorVersion = "1.19"

	// DefaultPSP defines the system level default PSP to apply
	DefaultPSP = "00-k0s-privileged"

	// Image Constants
	KonnectivityImage          = "us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent"
	KonnectivityImageVersion   = "v0.0.13"
	MetricsImage               = "gcr.io/k8s-staging-metrics-server/metrics-server"
	MetricsImageVersion        = "v0.3.7"
	KubeProxyImage             = "k8s.gcr.io/kube-proxy"
	KubeProxyImageVersion      = "v1.19.0"
	CoreDNSImage               = "docker.io/coredns/coredns"
	CoreDNSImageVersion        = "1.7.0"
	CalicoImage                = "calico/cni"
	CalicoImageVersion         = "v3.16.2"
	FlexVolumeImage            = "calico/pod2daemon-flexvol"
	FlexVolumeImageVersion     = "v3.16.2"
	CalicoNodeImage            = "calico/node"
	CalicoNodeImageVersion     = "v3.16.2"
	KubeControllerImage        = "calico/kube-controllers"
	KubeControllerImageVersion = "v3.16.2"

	// Helm constants
	HelmHome             = "C:\\var\\lib\\k0s\\helmhome"
	HelmRepositoryConfig = "C:\\var\\lib\\k0s\\helmhome/repositories.yaml"
	HelmRepositoryCache  = "C:\\var\\lib\\k0s\\helmhome/cache"
)
