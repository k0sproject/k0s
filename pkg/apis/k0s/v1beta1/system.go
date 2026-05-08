// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import "github.com/k0sproject/k0s/pkg/constant"

// System users used to run controller components. Users will be created when
// running `k0s install`. They will own the generated certificates and
// kubeconfigs and be used to execute the supervised processes. If they don't
// exist, k0s will fallback to the root user.
type SystemUser struct {
	// User to use for managed etcd (default "etcd")
	Etcd string `json:"etcdUser,omitempty"`
	// User to use for kine (default "kube-apiserver")
	Kine string `json:"kineUser,omitempty"`
	// User to use for the konnectivity server (default "konnectivity-server")
	Konnectivity string `json:"konnectivityUser,omitempty"`
	// User to use for the Kubernetes API Server (default "kube-apiserver")
	KubeAPIServer string `json:"kubeAPIserverUser,omitempty"`
	// User to use for the Kubernetes scheduler (default "kube-scheduler")
	KubeScheduler string `json:"kubeSchedulerUser,omitempty"`
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
