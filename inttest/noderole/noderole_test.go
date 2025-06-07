/*
Copyright 2021 k0s authors

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

package noderole

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type NodeRoleSuite struct {
	*common.BootlooseSuite
}

func (s *NodeRoleSuite) TestK0sGetsUp() {
	if s.ControllerCount == 1 {
		s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server", "--single"))
	} else if s.ControllerCount > 1 {
		s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server", "--enable-worker", "--no-taints"))

		token, err := s.GetJoinToken("controller")
		s.Require().NoError(err)
		for idx := 1; idx < s.ControllerCount; idx++ {
			s.Require().NoError(s.InitController(idx, "--disable-components=konnectivity-server,metrics-server", "--enable-worker", token))
		}
	}

	if s.WorkerCount > 0 {
		s.Require().NoError(s.RunWorkers())
	}

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Wait a bit to see if any unwanted stuff pops up.
	deadline := time.Now().Add(2 * time.Minute)
	s.T().Logf("Observing the cluster for two minutes until %s ...", deadline.Format(time.TimeOnly))
	nodes := kc.CoreV1().Nodes()
	labelledNodes := make(map[string]struct{})
	taintedNodes := make(map[string]struct{})
	success := errors.New("observation period ended")
	ctx, cancel := context.WithDeadlineCause(s.Context(), deadline, success)
	defer cancel()
	err = watch.Nodes(nodes).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(node *corev1.Node) (bool, error) {
			if value, exists := node.Labels[constants.LabelNodeRoleControlPlane]; exists {
				if value != "true" {
					return false, fmt.Errorf("unexpected control-plane label value for %s: %q", node.Name, value)
				}
				labelledNodes[node.Name] = struct{}{}
			} else if _, wasLabelled := labelledNodes[node.Name]; wasLabelled {
				return false, errors.New("control-plane label has been removed from node " + node.Name)
			}

			idx := slices.IndexFunc(node.Spec.Taints, func(taint corev1.Taint) bool {
				return taint.Key == constants.LabelNodeRoleControlPlane
			})
			if idx >= 0 {
				if node.Name == s.ControllerNode(0) {
					return false, fmt.Errorf("node %s should never be tainted: %v", node.Name, node.Spec.Taints[idx])
				}
				if node.Spec.Taints[idx] != constants.ControlPlaneTaint {
					return false, fmt.Errorf("unexpected taint for node %s: %v", node.Name, node.Spec.Taints[idx])
				}
				taintedNodes[node.Name] = struct{}{}
			} else if _, wasTainted := taintedNodes[node.Name]; wasTainted {
				return false, fmt.Errorf("taint for node %s has been removed", node.Name)
			}

			return false, nil
		})
	s.Require().Error(err)
	if !errors.Is(err, success) {
		s.FailNow(err.Error())
	}

	for idx := range s.ControllerCount {
		nodeName := s.ControllerNode(idx)
		s.Contains(labelledNodes, nodeName, "missing control-plane label for %s", nodeName)
		if idx > 0 {
			s.Contains(taintedNodes, nodeName, "missing control-plane taint for %s", nodeName)
		}
	}
}

func TestNodeRoleSuite(t *testing.T) {
	s := &common.BootlooseSuite{
		ControllerCount: 2,
		WorkerCount:     1,
	}

	if os.Getenv("K0S_TEST_TARGET") == "noderole-single" {
		s = &common.BootlooseSuite{ControllerCount: 1}
	}

	suite.Run(t, &NodeRoleSuite{s})
}
