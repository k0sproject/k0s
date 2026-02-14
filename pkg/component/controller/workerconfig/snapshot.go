// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	pauseWindowsImage      *v1beta1.ImageSpec
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
	out.featureGates = s.featureGates.DeepCopy()
	out.pauseImage = s.pauseImage.DeepCopy()
	out.pauseWindowsImage = s.pauseWindowsImage.DeepCopy()
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
		spec.Images.Windows.Pause.DeepCopy(),
	}
}
