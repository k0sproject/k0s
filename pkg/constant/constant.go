package constant

const (
	// DataDir folder contains all mke state
	DataDir = "/var/lib/mke"

	// CertRoot defines the root location for all pki related artifacts
	CertRoot = "/var/lib/mke/pki"

	// KubeletBootstrapConfigPath defines the default path for kubelet bootstrap auth config
	KubeletBootstrapConfigPath = "/var/lib/mke/kubelet-bootstrap.conf"

	// AdminKubeconfigConfigPath defines the cluster admin kubeconfig location
	AdminKubeconfigConfigPath = "/var/lib/mke/pki/admin.conf"

	// PidDir defines the location of supervised pid files
	PidDir = "/run/mke"

	// Group defines group name for shared directories
	Group = "mke"

	// User accounts for services
	EtcdUser              = "etcd"
	ApiserverUser         = "kube-apiserver"
	ControllerManagerUser = "kube-controller-manager"
	SchedulerUser         = "kube-scheduler"
)
