package constant

const (
	// DataDir folder contains all mke state
	DataDir = "/var/lib/mke"
	// DataDirMode is the expected directory permissions for DataDir
	DataDirMode = 0755
	// CertRoot defines the root location for all pki related artifacts
	CertRoot = "/var/lib/mke/pki"
	// CertRootMode is the expected directory permissions for CertRoot. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.20/
	CertRootMode = 0644
	// CertRootSecureMode is the expected file permissions for secure files. see: https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.13/
	// this relates to files like: admin.conf, kube-apiserver.yaml, certificate files, and more
	CertRootSecureMode = 0640
	// BinDir defines the location for all pki related binaries
	BinDir = "/var/lib/mke/bin"
	// BinDirMode is the expected directory permissions for BinDir
	BinDirMode = 0755
	// RunDir defines the location of supervised pid files and sockets
	RunDir = "/run/mke"
	// ManifestsDir defines the location for all stack manifests
	ManifestsDir = "/var/lib/mke/manifests"
	// ManifestsDirMode is the expected directory permissions for ManifestsDir
	ManifestsDirMode = 0644

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "/var/lib/mke/kubelet-bootstrap.conf"
	// KubeletAuthConfigPath defines the default kubelet auth config path
	KubeletAuthConfigPath = "/var/lib/mke/kubelet.conf"
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
)
