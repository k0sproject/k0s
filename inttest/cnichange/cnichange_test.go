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

package cnichange

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
)

type CNIChangeSuite struct {
	common.BootlooseSuite
}

func (s *CNIChangeSuite) TestK0sGetsUpButRejectsToChangeCNI() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithKubeRouter)

	// Run controller with defaults only --> kube-router in use
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Wait till we see kube-router DS created
	err = watch.DaemonSets(kc.AppsV1().DaemonSets("kube-system")).
		WithObjectName("kube-router").
		Until(s.Context(), func(ds *appsv1.DaemonSet) (bool, error) {
			return true, nil
		})
	s.Require().NoError(err)

	// Restart the controller with new config, should keep kube-router still in use
	sshC1, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer sshC1.Disconnect()

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithCalico)

	s.T().Log("restarting k0s")
	_, err = sshC1.ExecWithOutput(s.Context(), "rc-service k0scontroller restart")
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

	// check that we see the expeted warning event
	err = watch.Events(kc.CoreV1().Events("kube-system")).
		WithFieldSelector(fields.ParseSelectorOrDie("involvedObject.name=k0s")).
		Until(s.Context(), func(e *corev1.Event) (bool, error) {
			return e.Type == "Warning" && strings.Contains(e.Message, "cannot change CNI provider from kuberouter to calico"), nil
		})
	s.Require().NoError(err)
}

func TestCNIChangeSuite(t *testing.T) {
	s := CNIChangeSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithCalico = `
spec:
  network:
    provider: calico
    calico:
`

const k0sConfigWithKubeRouter = `
spec:
  network:
    provider: kuberouter
`
