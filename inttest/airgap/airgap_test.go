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
	"context"
	"github.com/stretchr/testify/suite"
	"github.com/weaveworks/footloose/pkg/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
)

type AirgapSuite struct {
	common.FootlooseSuite
}

func (s *AirgapSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0))
	s.NoError(s.RunWorkers(`--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args="--address=0.0.0.0 --event-burst=10"`))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	labels, err := s.GetNodeLabels(s.WorkerNode(0), kc)
	s.NoError(err)
	s.Equal("bar", labels["k0sproject.io/foo"])

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(kc), "kube-router did not start")
	
	// at that moment we can assume that all pods has at least started
	events, err := kc.CoreV1().Events("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)
	imagesUsed := 0
	imagesPulled := 0
	pulledImagesMessages := []string{}
	for _, event := range events.Items {
		if strings.Contains(event.Message, "already present on machine") {
			imagesUsed++
		}
		if strings.Contains(event.Message, "Pulling image") {
			pulledImagesMessages = append(pulledImagesMessages, event.Message)
			imagesPulled++
		}
	}
	s.T().Logf("Used %d images from airgap bundle", imagesUsed)
	if len(pulledImagesMessages) > 0 {
		s.T().Logf("Image pulls messages")
		for _, message := range pulledImagesMessages {
			s.T().Logf(message)
		}
	}
	s.Require().Equal(imagesPulled, 0)

}

func TestAirgapSuite(t *testing.T) {
	s := AirgapSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			ExtraVolumes: []config.Volume{
				{
					Type:        "bind",
					Source:      os.Getenv("K0S_IMAGES_BUNDLE"),
					Destination: "/var/lib/k0s/images/bundle.tar",
				},
			},
		},
	}
	suite.Run(t, &s)
}
