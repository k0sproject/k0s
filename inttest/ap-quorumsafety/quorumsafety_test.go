// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package quorumsafety

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"
	"github.com/stretchr/testify/suite"
)

type quorumSafetySuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *quorumSafetySuite) SetupTest() {
	ctx := s.Context()
	var joinToken string

	k0sConfig := "spec: {api: {externalAddress: " + s.GetLBAddress() + "}}"
	for idx := range s.ControllerCount {
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", k0sConfig)

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		client, err := s.ExtensionsClient(s.ControllerNode(0))
		s.Require().NoError(err)

		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))

		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := range s.ControllerCount {
		s.Require().Len(s.GetMembers(idx), s.ControllerCount)
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *quorumSafetySuite) TestApply() {
	ctx, cancelTest := context.WithCancelCause(s.Context())
	defer cancelTest(nil)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)

	stoppedController := s.ControllerNode(2)
	s.Require().NoError(s.StopController(stoppedController))
	s.T().Log(stoppedController, "stopped")

	var (
		controllerRestarted atomic.Bool
		planState           value.Latest[apv1beta2.PlanStateType]
	)

	var wg sync.WaitGroup
	s.T().Cleanup(wg.Wait)

	wg.Go(func() {
		s.T().Log("Monitoring Plan to reach the", appc.PlanCompleted, "state")

		err := watch.Plans(client.AutopilotV1beta2().Plans()).
			WithObjectName(apconst.AutopilotName).
			WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
			Until(ctx, func(plan *apv1beta2.Plan) (bool, error) {
				switch plan.Status.State {
				case appc.PlanSchedulable, appc.PlanSchedulableWait, "":
				case appc.PlanCompleted:
					if !controllerRestarted.Load() {
						return false, errors.New("Plan execution completed too early")
					}
				default:
					return false, fmt.Errorf("unexpected plan state: %s", plan.Status.State)
				}

				lastPlanState, _ := planState.Peek()
				if lastPlanState != plan.Status.State {
					s.T().Logf("Plan state changed: %s", plan.Status.State)
					planState.Set(plan.Status.State)
				}

				return plan.Status.State == appc.PlanCompleted, nil
			})

		if !s.NoErrorf(err, "While monitoring Plan to reach the %s state", appc.PlanCompleted) {
			cancelTest(fmt.Errorf("failed to monitor Plan to reach the %s state", appc.PlanCompleted))
		}
	})

	wg.Go(func() {
		s.T().Log("Monitoring ControlNodes to reach the", apsigcomm.Completed, "phase")

		lastStatuses := make(map[string]*apsigv2.Status)
		err := watch.ControlNodes(client.AutopilotV1beta2().ControlNodes()).
			WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
			Until(ctx, func(node *apv1beta2.ControlNode) (bool, error) {
				lastStatus := lastStatuses[node.Name]
				var signalData apsigv2.SignalData
				if err := signalData.Unmarshal(node.Annotations); err != nil {
					if lastStatus != nil {
						return false, fmt.Errorf("failed to unmarshal signal data of %s, last observed signal status was %v: %w", node.Name, lastStatus, err)
					}
					return false, nil
				}

				status := cmp.Or(signalData.Status, new(apsigv2.Status))
				if reflect.DeepEqual(lastStatus, status) {
					return false, nil
				}

				lastStatuses[node.Name] = status

				if !controllerRestarted.Load() {
					return false, fmt.Errorf("signal status of %s is %v, albeit %s hasn't been restarted yet", node.Name, status, stoppedController)
				}

				s.T().Log(node.Name, "signal status:", status)

				if len(lastStatuses) != s.ControllerCount {
					return false, nil
				}

				for _, status := range lastStatuses {
					if status.Status != apsigcomm.Completed {
						return false, nil
					}
				}

				return true, nil
			})

		if !s.NoErrorf(err, "While monitoring ControlNodes to reach the %s phase", apsigcomm.Completed) {
			cancelTest(fmt.Errorf("failed to monitor ControlNodes to reach the %s phase", apsigcomm.Completed))
		}
	})

	// Create + populate the plan

	_, err = client.AutopilotV1beta2().Plans().Create(ctx, &apv1beta2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: apconst.AutopilotName,
		},
		Spec: apv1beta2.PlanSpec{
			ID:        s.T().Name(),
			Timestamp: "now",
			Commands: []apv1beta2.PlanCommand{{
				K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
					Version:     "v0.0.0",
					ForceUpdate: true,
					Platforms: apv1beta2.PlanPlatformResourceURLMap{
						runtime.GOOS + "-" + runtime.GOARCH: apv1beta2.PlanResourceURL{URL: "http://localhost/dist/k0s"},
					},
					Targets: apv1beta2.PlanCommandTargets{
						Controllers: apv1beta2.PlanCommandTarget{
							Discovery: apv1beta2.PlanCommandTargetDiscovery{
								Static: &apv1beta2.PlanCommandTargetDiscoveryStatic{
									Nodes: func() (nodes []string) {
										for idx := range s.ControllerCount {
											nodes = append(nodes, s.ControllerNode(idx))
										}
										return nodes
									}(),
								},
							},
						},
					},
				}},
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.T().Log("Plan created")

	s.T().Log("Waiting for Plan to settle in", appc.PlanSchedulable, "state for at least 10 seconds")
	for {
		planState, planStateChanged := planState.Peek()
		select {
		case <-ctx.Done():
			s.Require().Fail("Test canceled", "%v", context.Cause(ctx))

		case <-planStateChanged:
			continue

		case <-time.After(10 * time.Second):
			if planState != appc.PlanSchedulable {
				continue
			}

			s.T().Log("Restarting", stoppedController, "after Plan remained in", planState, "state for at least 10 seconds")
			controllerRestarted.Store(true) // Store it before actually starting the controller to prevent races.
			s.Require().NoErrorf(s.StartController(stoppedController), "Failed to restart %s", stoppedController)
		}

		break
	}

	wg.Wait()
}

// TestQuorumSafetySuite sets up a suite using 3 controllers, and runs a specific
// test scenario covering the breaking of quorum.
func TestQuorumSafetySuite(t *testing.T) {
	suite.Run(t, &quorumSafetySuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
			WithLB:          true,
		},
	})
}
