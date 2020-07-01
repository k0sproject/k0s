package constant

const (
	// DataDir folder contains all mke state
	DataDir = "/var/lib/mke"

	// CertRoot defines the root location for all pki related artifacts
	CertRoot = "/var/lib/mke/pki"

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "/var/lib/mke/kubelet-bootstrap.conf"

	// PidDir defines the location of supervised pid files
	PidDir = "/run/mke"
)
