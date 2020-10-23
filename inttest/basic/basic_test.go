/*
Copyright 2020 Mirantis, Inc.

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
package basic

import (
	"context"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"

	"github.com/Mirantis/mke/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BasicSuite struct {
	common.FootlooseSuite
}

func (s *BasicSuite) TestMkeGetsUp() {
	s.NoError(s.InitMainController())
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.Nil(s.WaitForCalicoReady(kc), "calico did not start")
}

func (s *BasicSuite) WaitForCalicoReady(kc *kubernetes.Clientset) error {
	s.T().Log("waiting to see calico ready in kube API")
	return wait.PollImmediate(1*time.Second, 5*time.Minute, func() (done bool, err error) {
		ds, err := kc.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "calico-node", v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
}

func TestBasicSuite(t *testing.T) {
	s := BasicSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
