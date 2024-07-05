/*
Copyright 2022 k0s authors

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

package workerconfig

import (
	"slices"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

// snapshot holds a snapshot of the parts that influence worker configurations.
type snapshot struct {

	// The snapshot of the cluster configuration.
	*configSnapshot

	// The list of API server addresses that are currently running.
	apiServers []net.HostPort

	// A simple counter that can be incremented every time a reconciliation
	// shall be enforced, even if the rest of the snapshot still matches the
	// last reconciled state.
	serial uint
}

// configSnapshot holds a snapshot of the parts of the cluster config spec that
// influence worker configurations.
type configSnapshot struct {
	dualStackEnabled       bool
	nodeLocalLoadBalancing *v1beta1.NodeLocalLoadBalancing
	konnectivityAgentPort  uint16
	defaultImagePullPolicy corev1.PullPolicy
	profiles               v1beta1.WorkerProfiles
	featureGates           v1beta1.FeatureGates
	pauseImage             *v1beta1.ImageSpec
}

func (s *snapshot) DeepCopy() *snapshot {
	if s == nil {
		return nil
	}
	out := new(snapshot)
	s.DeepCopyInto(out)
	return out
}

func (s *snapshot) DeepCopyInto(out *snapshot) {
	*out = *s
	out.apiServers = slices.Clone(s.apiServers)
	out.configSnapshot = s.configSnapshot.DeepCopy()
}

func (s *configSnapshot) DeepCopy() *configSnapshot {
	if s == nil {
		return nil
	}
	out := new(configSnapshot)
	s.DeepCopyInto(out)
	return out
}

func (s *configSnapshot) DeepCopyInto(out *configSnapshot) {
	*out = *s
	out.nodeLocalLoadBalancing = s.nodeLocalLoadBalancing.DeepCopy()
	out.profiles = s.profiles.DeepCopy()
}

// takeConfigSnapshot converts ClusterSpec to a delta snapshot
func takeConfigSnapshot(spec *v1beta1.ClusterSpec) configSnapshot {
	var konnectivityAgentPort uint16
	if spec.Konnectivity != nil {
		konnectivityAgentPort = uint16(spec.Konnectivity.AgentPort)
	} else {
		konnectivityAgentPort = uint16(v1beta1.DefaultKonnectivitySpec().AgentPort)
	}

	return configSnapshot{
		spec.Network.DualStack.Enabled,
		spec.Network.NodeLocalLoadBalancing.DeepCopy(),
		konnectivityAgentPort,
		corev1.PullPolicy(spec.Images.DefaultPullPolicy),
		spec.WorkerProfiles.DeepCopy(),
		spec.FeatureGates.DeepCopy(),
		spec.Images.Pause.DeepCopy(),
	}
}
