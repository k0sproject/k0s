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

package airgap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
)

const k0sConfig = `
spec:
  images:
    default_pull_policy: Never
`

type AirgapSuite struct {
	common.FootlooseSuite
}

const network = "airgap"

// SetupSuite creates the required network before starting footloose.
func (s *AirgapSuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *AirgapSuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.MaybeDestroyNetwork(network))
}

func (s *AirgapSuite) TestK0sGetsUp() {
	(&common.Airgap{
		SSH:  s.SSH,
		Logf: s.T().Logf,
	}).LockdownMachines(s.Context(),
		s.ControllerNode(0), s.WorkerNode(0),
	)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.NoError(s.RunWorkers(`--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args="--address=0.0.0.0 --event-burst=10 --image-gc-high-threshold=100"`))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	if labels, err := s.GetNodeLabels(s.WorkerNode(0), kc); s.NoError(err) {
		s.Equal("bar", labels["k0sproject.io/foo"])
	}

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	// at that moment we can assume that all pods has at least started
	events, err := kc.CoreV1().Events("kube-system").List(s.Context(), v1.ListOptions{
		Limit: 100,
	})
	s.Require().NoError(err)
	imagesUsed := 0
	var pulledImagesMessages []string
	for _, event := range events.Items {
		if event.Source.Component == "kubelet" && event.Reason == "Pulled" {
			// We're interested only in image pull events
			s.T().Logf(event.Message)
			if strings.Contains(event.Message, "already present on machine") {
				imagesUsed++
			}
			if strings.Contains(event.Message, "Pulling image") {
				pulledImagesMessages = append(pulledImagesMessages, event.Message)
			}
		}
	}
	s.T().Logf("Used %d images from airgap bundle", imagesUsed)
	if len(pulledImagesMessages) > 0 {
		s.T().Logf("Image pulls messages")
		for _, message := range pulledImagesMessages {
			s.T().Logf(message)
		}
		s.Fail("Require all images be installed from bundle")
	}
}

func TestAirgapSuite(t *testing.T) {
	s := AirgapSuite{
		common.FootlooseSuite{
			ControllerCount:    1,
			WorkerCount:        1,
			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},

			AirgapImageBundleMountPoints: []string{"/var/lib/k0s/images/bundle.tar"},
		},
	}
	suite.Run(t, &s)
}
