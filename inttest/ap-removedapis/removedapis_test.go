// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package removedapis

import (
	"testing"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	corev1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type plansRemovedAPIsSuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansRemovedAPIsSuite) SetupTest() {
	ctx := s.Context()

	s.Require().NoError(s.InitController(0, "--single --disable-components=metrics-server"))

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	// Create the test CRD
	extClient, err := apiextensionsv1client.NewForConfig(restConfig)
	s.Require().NoError(err)
	removedCRD, err := extClient.CustomResourceDefinitions().Create(ctx, &removedCRD, corev1.CreateOptions{})
	s.Require().NoError(err)

	s.T().Log("Waiting for the CRDs to be established")
	s.Require().NoError(aptest.WaitForCRDByName(ctx, extClient, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, extClient, "controlnodes"))
	s.Require().NoError(aptest.WaitForCRDByGroupName(ctx, extClient, removedCRD.Name))

	// Create a resource for the test CRD
	dynClient, err := dynamic.NewForConfig(restConfig)
	s.Require().NoError(err)
	_, err = dynClient.Resource(schema.GroupVersionResource{
		Group:    removedCRD.Spec.Group,
		Version:  removedCRD.Spec.Versions[0].Name,
		Resource: removedCRD.Spec.Names.Plural,
	}).Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": removedCRD.Spec.Group + "/" + removedCRD.Spec.Versions[0].Name,
		"kind":       removedCRD.Spec.Names.Kind,
		"metadata": corev1.ObjectMeta{
			Name: "removed-resource",
		},
	}}, corev1.CreateOptions{})
	s.Require().NoError(err)
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansRemovedAPIsSuite) TestApply() {
	ctx := s.Context()

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	_, err = common.Create(ctx, restConfig, []byte(planTemplate))
	s.Require().NoError(err)
	s.T().Logf("Plan created")

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)
	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanWarning)
	if s.NoError(err) && s.Len(plan.Status.Commands, 1) {
		s.Equal(appc.PlanWarning, plan.Status.Commands[0].State)
		s.Equal(removedCRD.Name+" "+removedCRD.Spec.Versions[0].Name+" has been removed in Kubernetes v99.99.99, but there are 1 such resources in the cluster", plan.Status.Commands[0].Description)
	}
}

// TestPlansRemovedAPIsSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansRemovedAPIsSuite(t *testing.T) {
	suite.Run(t, &plansRemovedAPIsSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}

const planTemplate = `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id123
  timestamp: now
  commands:
    - k0supdate:
        version: v99.99.99
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
          linux-arm64:
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`

var removedCRD = apiextensionsv1.CustomResourceDefinition{
	ObjectMeta: corev1.ObjectMeta{
		Name: "removedcrds.k0s.k0sproject.example.com",
	},
	Spec: apiextensionsv1.CustomResourceDefinitionSpec{
		Group: "k0s.k0sproject.example.com",
		Names: apiextensionsv1.CustomResourceDefinitionNames{
			Kind: "RemovedCRD", Singular: "removedcrd", Plural: "removedcrds",
		},
		Scope: apiextensionsv1.ClusterScoped,
		Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
			Name: "v1beta1", Served: true, Storage: true,
			Schema: &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object",
				},
			},
		}},
	},
}
