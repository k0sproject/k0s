package v1beta1

import "github.com/k0sproject/k0s/pkg/constant"

// SystemUser defines the user to use for each component
type SystemUser struct {
	Etcd          string `yaml:"etcdUser,omitempty"`
	Kine          string `yaml:"kineUser,omitempty"`
	Konnectivity  string `yaml:"konnectivityUser,omitempty"`
	KubeAPIServer string `yaml:"kubeAPIserverUser,omitempty"`
	KubeScheduler string `yaml:"kubeSchedulerUser,omitempty"`
}

// DefaultSystemUsers returns the default system users to be used for the different components
func DefaultSystemUsers() *SystemUser {
	return &SystemUser{
		Etcd:          constant.EtcdUser,
		Kine:          constant.KineUser,
		Konnectivity:  constant.KonnectivityServerUser,
		KubeAPIServer: constant.ApiserverUser,
		KubeScheduler: constant.SchedulerUser,
	}
}

// DefaultInstallSpec ...
func DefaultInstallSpec() *InstallSpec {
	return &InstallSpec{
		SystemUsers: DefaultSystemUsers(),
	}
}
