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
	"context"
	"fmt"
	"testing"

	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"
	ts "github.com/k0sproject/k0s/inttest/toolsuite"
	tsops "github.com/k0sproject/k0s/inttest/toolsuite/operations"
	tsutil "github.com/k0sproject/k0s/inttest/toolsuite/util"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type VersionUpdateSuite struct {
	ts.ToolSuite
}

// TestUpdate uses the cluster kubeconfig to wait for the Plan to complete successfully.
func (s *VersionUpdateSuite) TestUpdate() {
	client, err := s.AutopilotClient()
	assert.NoError(s.T(), err)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	ctx, cancel := context.WithTimeout(s.Context(), s.Config.OperationTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	s.T().Logf("Waiting for Plan completion (deadline = %s)", deadline)
	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	if s.NoError(err) {
		s.NotEmpty(plan)
	}
}

// VersionUpdatePlan creates an autopilot Plan that consists of all controllers and workers as
// reported by the terraform output. All updates are done sequentially, using the `linux-amd64` k0s
// distribution hosted on the GitHub project release page.
func VersionUpdatePlan(output tsutil.TerraformOutputMap) (*apv1beta2.Plan, error) {
	var updateK0sVersion string
	if err := tsutil.TerraformOutput(&updateK0sVersion, output, "k0s_update_version"); err != nil {
		return nil, err
	}

	var updateK0sURL string
	if err := tsutil.TerraformOutput(&updateK0sURL, output, "k0s_update_binary_url"); err != nil {
		return nil, err
	}

	// The k0s filename is URL encoded in the path, but the filename is not.

	var controllers []string
	if err := tsutil.TerraformOutput(&controllers, output, "k0s_controllers"); err != nil {
		return nil, fmt.Errorf("unable to parse terraform output for 'k0s_controllers': %w", err)
	}

	var workers []string
	if err := tsutil.TerraformOutput(&workers, output, "k0s_workers"); err != nil {
		return nil, fmt.Errorf("unable to parse terraform output for 'k0s_workers': %w", err)
	}

	return &apv1beta2.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "autopilot.k0sproject.io/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1beta2.PlanSpec{
			ID:        "abc123",
			Timestamp: "now",
			Commands: []apv1beta2.PlanCommand{
				{
					K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
						Version:     updateK0sVersion,
						ForceUpdate: true,
						Platforms: apv1beta2.PlanPlatformResourceURLMap{
							"linux-amd64": apv1beta2.PlanResourceURL{
								URL: updateK0sURL,
							},
						},
						Targets: apv1beta2.PlanCommandTargets{
							Controllers: apv1beta2.PlanCommandTarget{
								Discovery: apv1beta2.PlanCommandTargetDiscovery{
									Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
										Nodes: controllers,
									},
								},
							},
							Workers: apv1beta2.PlanCommandTarget{
								Discovery: apv1beta2.PlanCommandTargetDiscovery{
									Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
										Nodes: workers,
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// TestVersionUpdateSuite runs the VersionUpdateSuite suite.
func TestVersionUpdateSuite(t *testing.T) {
	suite.Run(t, &VersionUpdateSuite{
		ts.ToolSuite{
			Operation: tsops.PlanApplyOperation(VersionUpdatePlan),
		},
	})
}
