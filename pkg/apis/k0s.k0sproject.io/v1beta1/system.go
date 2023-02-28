/*
Copyright 2020 k0s authors

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

package v1beta1

import "github.com/k0sproject/k0s/pkg/constant"

// SystemUser defines the user to use for each component
type SystemUser struct {
	Etcd          string `json:"etcdUser,omitempty"`
	Kine          string `json:"kineUser,omitempty"`
	Konnectivity  string `json:"konnectivityUser,omitempty"`
	KubeAPIServer string `json:"kubeAPIserverUser,omitempty"`
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
