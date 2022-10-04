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
	"context"
	"fmt"
	"testing"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
)

type plansRemovedAPIsSuite struct {
	common.FootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansRemovedAPIsSuite) SetupTest() {
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))

	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	s.MakeDir(s.ControllerNode(0), "/var/lib/k0s/manifests/test")
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/crd.yaml", testCRD)

	client, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	_, err = apcomm.WaitForCRDByName(context.TODO(), client, "certificates.cert-manager.io", 2*time.Minute)
	s.Require().NoError(err)

	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/cert.yaml", testManifest)

	_, perr := apcomm.WaitForCRDByName(context.TODO(), client, "plans.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(perr)
	_, cerr := apcomm.WaitForCRDByName(context.TODO(), client, "controlnodes.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(cerr)
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansRemovedAPIsSuite) TestApply() {

	manifestFile := "/tmp/plan.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := apcomm.WaitForPlanByName(context.TODO(), client, apconst.AutopilotName, 5*time.Minute, func(obj interface{}) bool {
		if plan, ok := obj.(*apv1beta2.Plan); ok {
			return plan.Status.State == appc.PlanWarning
		}

		return false
	})

	s.NoError(err)
	s.Equal(appc.PlanWarning, plan.Status.State)

	s.Equal(1, len(plan.Status.Commands))
	cmd := plan.Status.Commands[0]

	s.Equal(appc.PlanWarning, cmd.State)
}

// TestPlansRemovedAPIsSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansRemovedAPIsSuite(t *testing.T) {
	suite.Run(t, &plansRemovedAPIsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
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
        version: v1.26.0
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`

const testManifest = `
apiVersion: cert-manager.io/v1beta1
kind: Certificate
metadata:
  name: test-com
  namespace: default
spec:
  secretName: test-com-tls
  issuerRef:
    name: test-issuer
    kind: Issuer
    group: cert-manager.io
`

const testCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
  labels:
    app: 'cert-manager'
    app.kubernetes.io/name: 'cert-manager'
    app.kubernetes.io/instance: 'cert-manager'
spec:
  group: cert-manager.io
  names:
    kind: Certificate
    listKind: CertificateList
    plural: certificates
    shortNames:
      - cert
      - certs
    singular: certificate
    categories:
      - cert-manager
  scope: Namespaced
  versions:
    - name: v1beta1
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          description: "A Certificate resource should be created to ensure an up to date and signed x509 certificate is stored in the Kubernetes Secret resource named in spec.secretName"
          type: object
          required:
            - spec
          properties:
            apiVersion:
              description: 'APIVersion'
              type: string
            kind:
              description: 'kind'
              type: string
            metadata:
              type: object
            spec:
              description: Desired state of the Certificate resource.
              type: object
              required:
                - issuerRef
                - secretName
              properties:
                issuerRef:
                  description: IssuerRef is a reference to the issuer for this certificate. 
                  type: object
                  required:
                    - name
                  properties:
                    group:
                      description: Group of the resource being referred to.
                      type: string
                    kind:
                      description: Kind of the resource being referred to.
                      type: string
                    name:
                      description: Name of the resource being referred to.
                      type: string
                secretName:
                  description: SecretName is the name of the secret resource that will be automatically created and managed by this Certificate resource. It will be populated with a private key and certificate, signed by the denoted issuer.
                  type: string
            status:
              description: Status of the Certificate. This is set and managed automatically.
              type: object
              properties:
                conditions:
                  description: List of status conditions to indicate the status of certificates.
                  type: array
                  items:
                    description: CertificateCondition contains condition information for an Certificate.
                    type: object
                    required:
                      - status
                      - type
                    properties:
                      status:
                        description: Status of the condition
                        type: string
                        enum:
                          - "True"
                          - "False"
                          - Unknown
                      type:
                        description: Type of the condition
                        type: string
                  x-kubernetes-list-map-keys:
                    - type
                  x-kubernetes-list-type: map
      served: true
      storage: true
`
