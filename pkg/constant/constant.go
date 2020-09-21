package constant

const (
	// DataDir folder contains all mke state
	DataDir = "/var/lib/mke"

	// CertRoot defines the root location for all pki related artifacts
	CertRoot = "/var/lib/mke/pki"

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "/var/lib/mke/kubelet-bootstrap.conf"
	// KubeletAuthConfigPath defines the default kubelet auth config path
	KubeletAuthConfigPath = "/var/lib/mke/kubelet.conf"

	// AdminKubeconfigConfigPath defines the cluster admin kubeconfig location
	AdminKubeconfigConfigPath = "/var/lib/mke/pki/admin.conf"

	// RunDir defines the location of supervised pid files and sockets
	RunDir = "/run/mke"

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
