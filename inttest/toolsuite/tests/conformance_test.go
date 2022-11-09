// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"flag"
	"fmt"
	"testing"

	ts "github.com/k0sproject/k0s/inttest/toolsuite"
	tsops "github.com/k0sproject/k0s/inttest/toolsuite/operations"

	"github.com/stretchr/testify/suite"
)

type ConformanceConfig struct {
	KubernetesVersion string
}

type ConformanceSuite struct {
	ts.ToolSuite
}

var config ConformanceConfig

func init() {
	flag.StringVar(&config.KubernetesVersion, "conformance-kubernetes-version", "", "The kubernetes version of the conformance tests to run")
}

// TestConformanceSuite runs the Sonobuoy conformance tests for a specific k8s version.
func TestConformanceSuite(t *testing.T) {
	if config.KubernetesVersion == "" {
		t.Fatal("--conformance-kubernetes-version is a required parameter")
	}

	suite.Run(t, &ConformanceSuite{
		ts.ToolSuite{
			Operation: tsops.SonobuoyOperation(
				tsops.SonobuoyConfig{
					Parameters: []string{
						"--mode=certified-conformance",
						fmt.Sprintf("--kubernetes-version=%s", config.KubernetesVersion),
					},
				},
			),
		},
	})
}
