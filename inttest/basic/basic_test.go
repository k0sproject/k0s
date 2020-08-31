package basic

import (
	"testing"

	"github.com/Mirantis/mke/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BasicSuite struct {
	common.FootlooseSuite
}

func (s *BasicSuite) TestMkeGetsUp() {
	s.NoError(s.RunControllers())
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

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
