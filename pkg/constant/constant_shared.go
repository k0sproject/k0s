package constant

import "fmt"

// WinCertCA defines the CA.crt location.
// this one is defined here because it is used not only on windows worker but also during the control plane bootstrap
const WinCertCA = "C:\\var\\lib\\k0s\\pki\\ca.crt"

// CfgVars is a struct that holds all the config variables requried for K0s
type CfgVars struct {
	AdminKubeConfigPath        string // The cluster admin kubeconfig location
	BinDir                     string // location for all pki related binaries
	CertRootDir                string // CertRootDir defines the root location for all pki related artifacts
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
		dataDir = DataDirDefault
	}

	runDir := fmt.Sprintf("%s/run", dataDir)
	certDir := fmt.Sprintf("%s/pki", dataDir)
	helmHome := fmt.Sprintf("%s/helmhome", dataDir)

	return CfgVars{
		AdminKubeConfigPath:        fmt.Sprintf("%s/admin.conf", certDir),
		BinDir:                     fmt.Sprintf("%s/bin", dataDir),
		CertRootDir:                certDir,
		DataDir:                    dataDir,
		EtcdCertDir:                fmt.Sprintf("%s/etcd", certDir),
		EtcdDataDir:                fmt.Sprintf("%s/etcd", dataDir),
		KineSocketPath:             fmt.Sprintf("%s/kine/kine.sock:2379", runDir),
		KonnectivitySocketDir:      fmt.Sprintf("%s/konnectivity-server", runDir),
		KubeletAuthConfigPath:      fmt.Sprintf("%s/kubelet.conf", dataDir),
		KubeletBootstrapConfigPath: fmt.Sprintf("%s/kubelet-bootstrap.conf", dataDir),
		KubeletVolumePluginDir:     "/usr/libexec/k0s/kubelet-plugins/volume/exec",
		ManifestsDir:               fmt.Sprintf("%s/manifests", dataDir),
		RunDir:                     runDir,
		KonnectivityKubeConfigPath: fmt.Sprintf("%s/konnectivity.conf", certDir),

		// Helm Config
		HelmHome:             helmHome,
		HelmRepositoryCache:  fmt.Sprintf("%s/cache", helmHome),
		HelmRepositoryConfig: fmt.Sprintf("%s/repositories.yaml", helmHome),
	}
}
