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
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/stretchr/testify/suite"
)

type CLITestSuite struct {
	suite.Suite
}

func (s *CLITestSuite) TestCustomImageList() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  images:
    konnectivity:
      image: custom-repository/my-custom-konnectivity-image
      version: v0.0.1
    coredns:
      image: custom.io/coredns/coredns
      version: 1.0.0
`
	cfg, err := v1beta1.ConfigFromString(yamlData, "")
	s.NoError(err)
	a := cfg.Spec.Images

	s.Equal("custom-repository/my-custom-konnectivity-image:v0.0.1", a.Konnectivity.URI())
	s.Equal("1.0.0", a.CoreDNS.Version)
	s.Equal("custom.io/coredns/coredns", a.CoreDNS.Image)
	s.Equal("k8s.gcr.io/metrics-server/metrics-server", a.MetricsServer.Image)
}

func TestCLITestSuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}
