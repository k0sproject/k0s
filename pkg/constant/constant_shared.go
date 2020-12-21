package constant

import (
	"runtime"
)

// WinCertCA defines the CA.crt location.
// this one is defined here because it is used not only on windows worker but also during the control plane bootstrap
const WinCertCA = "C:\\var\\lib\\k0s\\pki\\ca.crt"
const WinDataDirDefault = "C:\\var\\lib\\k0s"

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
	// KubeletVolumePlugindDirMode is the expected directory permissions for KubeleteVolumePluginDir
	KubeletVolumePluginDirMode = 0700

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
	// Image Constants
	KonnectivityImage          = "us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent"
	KonnectivityImageVersion   = "v0.0.13"
	MetricsImage               = "gcr.io/k8s-staging-metrics-server/metrics-server"
	MetricsImageVersion        = "v0.3.7"
	KubeProxyImage             = "k8s.gcr.io/kube-proxy"
	KubeProxyImageVersion      = "v1.20.1"
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

// CfgVars is a struct that holds all the config variables requried for K0s
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

	runDir := formatPath(dataDir, "run")
	certDir := formatPath(dataDir, "pki")
	winCertDir := WinDataDirDefault + "\\pki" // hacky but we need it to be windows style even on linux machine
	helmHome := formatPath(dataDir, "helmhome")

	return CfgVars{
		AdminKubeConfigPath:        formatPath(certDir, "admin.conf"),
		BinDir:                     formatPath(dataDir, "bin"),
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
