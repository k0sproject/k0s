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
	// DataDir folder contains all mke state
	DataDir = "/var/lib/mke"
	// DataDirMode is the expected directory permissions for DataDir
	DataDirMode = 0755
	// EtcdDataDir contains etcd state
	EtcdDataDir = "/var/lib/mke/etcd"
	// EtcdDataDirMode is the expected directory permissions for EtcdDataDir. see https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	EtcdDataDirMode = 0700
	// CertRootDir defines the root location for all pki related artifacts
	CertRootDir = "/var/lib/mke/pki"
	// CertRootDirMode is the expected directory permissions for CertRootDir.
	CertRootDirMode = 0750
	//EtcdCertDir contains etcd certificates
	EtcdCertDir = "/var/lib/mke/pki/etcd"
	// EtcdCertDirMode is the expected directory permissions for EtcdCertDir
	EtcdCertDirMode = 0700
	// CertMode is the expected permissions for certificates. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.20/
	CertMode = 0644
	// CertSecureMode is the expected file permissions for secure files. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.13/
	// this relates to files like: admin.conf, kube-apiserver.yaml, certificate files, and more
	CertSecureMode = 0640
	// BinDir defines the location for all pki related binaries
	BinDir = "/var/lib/mke/bin"
	// BinDirMode is the expected directory permissions for BinDir
	BinDirMode = 0755
	// RunDir defines the location of supervised pid files and sockets
	RunDir = "/run/mke"
	// RunDirMode is the expected permissions of RunDir
	RunDirMode = 0755
	// PidFileMode is the expected file permissions for pid files
	PidFileMode = 0644
	// ManifestsDir defines the location for all stack manifests
	ManifestsDir = "/var/lib/mke/manifests"
	// ManifestsDirMode is the expected directory permissions for ManifestsDir
	ManifestsDirMode = 0644

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "/var/lib/mke/kubelet-bootstrap.conf"
	// KubeletAuthConfigPath defines the default kubelet auth config path
	KubeletAuthConfigPath = "/var/lib/mke/kubelet.conf"
	// KubeletVolumePluginDir defines the location for kubelet plugins volume executables
	KubeletVolumePluginDir = "/usr/libexec/mke/kubelet-plugins/volume/exec"
	// KubeletVolumePlugindDirMode is the expected directory permissions for KubeleteVolumePluginDir
	KubeletVolumePluginDirMode = 0700

	// AdminKubeconfigConfigPath defines the cluster admin kubeconfig location
	AdminKubeconfigConfigPath = "/var/lib/mke/pki/admin.conf"

	// Group defines group name for shared directories
	Group = "mke"

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

	// KubernetesMajorMinorVersion defines the current embedded major.minor version info
	KubernetesMajorMinorVersion = "1.19"

	// DefaultPSP defines the system level default PSP to apply
	DefaultPSP = "00-mke-privileged"

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
)
