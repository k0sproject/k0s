// +build !windows

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

import "fmt"

const (
	// DataDirDefault is the default data directory containing k0s state
	DataDirDefault = "/var/lib/k0s"
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
	// KubeletVolumePlugindDirMode is the expected directory permissions for KubeleteVolumePluginDir
	KubeletVolumePluginDirMode = 0700

	KubeletVolumePluginDir = "/usr/libexec/k0s/kubelet-plugins/volume/exec"

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
	KubernetesMajorMinorVersion = "1.20"
	// DefaultPSP defines the system level default PSP to apply
	DefaultPSP = "00-k0s-privileged"
	KineSocket = "kine/kine.sock:2379"
	// Image Constants
	KonnectivityImage          = "us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent"
	KonnectivityImageVersion   = "v0.0.13"
	MetricsImage               = "gcr.io/k8s-staging-metrics-server/metrics-server"
	MetricsImageVersion        = "v0.3.7"
	KubeProxyImage             = "k8s.gcr.io/kube-proxy"
	KubeProxyImageVersion      = "v1.20.0"
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

func formatPath(dir string, file string) string {
	return fmt.Sprintf("%s/%s", dir, file)
}