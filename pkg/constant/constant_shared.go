package constant

import (
	"runtime"
)

// WinCertCA defines the CA.crt location.
// this one is defined here because it is used not only on windows worker but also during the control plane bootstrap
const WinCertCA = "C:\\var\\lib\\k0s\\pki\\ca.crt"
const WinDataDirDefault = "C:\\var\\lib\\k0s"

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
