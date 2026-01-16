//go:build unix

// Copyright 2024 k0s authors
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

package controllerworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigk0s "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/k0s"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/ptr"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type controllerworkerSuite struct {
	common.BootlooseSuite
}

// TODO: Update this test after the https://github.com/k0sproject/k0s/pull/4860 is merged, backported and released.
// 	Apply this commit to properly test controller+worker update process:
//	 https://github.com/makhov/k0s/commit/bf702a829f958b04b7a6119ff03960e90100d4c9

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *controllerworkerSuite) SetupTest() {
	ctx := s.Context()
	// ipAddress := s.GetControllerIPAddress(0)
	var joinToken string

	k0sConfig := "spec: {api: {externalAddress: " + s.GetLBAddress() + "}}"
	for idx := range s.BootlooseSuite.ControllerCount {
		nodeName, require := s.ControllerNode(idx), s.Require()
		ssh, err := s.SSH(ctx, nodeName)
		require.NoError(err)
		defer ssh.Disconnect()
		s.PutFile(nodeName, "/tmp/k0s.yaml", k0sConfig)

		// Note that the token is intentionally empty for the first controller
		args := []string{
			"--debug",
			"--enable-worker",
			"--config=/tmp/k0s.yaml",
		}
		if joinToken != "" {
			s.PutFile(nodeName, "/tmp/token", joinToken)
			args = append(args, "--token-file=/tmp/token")
		}
		out, err := ssh.ExecWithOutput(ctx, "cp -f /dist/k0s /usr/local/bin/k0s && /usr/local/bin/k0s install controller "+strings.Join(args, " "))
		if err != nil {
			s.T().Logf("error installing k0s: %s", out)
		}
		require.NoError(err)
		_, err = ssh.ExecWithOutput(ctx, "k0s start")
		require.NoError(err)
		// s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", "--enable-worker", joinToken))
		s.Require().NoError(s.WaitJoinAPI(nodeName))
		kc, err := s.KubeClient(nodeName)
		require.NoError(err)
		require.NoError(s.WaitForNodeReady(nodeName, kc))

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
	for idx := range s.BootlooseSuite.ControllerCount {
		s.Require().Len(s.GetMembers(idx), s.BootlooseSuite.ControllerCount)
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *controllerworkerSuite) TestApply() {
	ctx, cancelTest := context.WithCancelCause(s.TContext())

	cf := s.ClientFactory(s.ControllerNode(0))

	c, err := cf.GetClient()
	s.Require().NoError(err)

	// Create a Deployment plus PDB that will block node draining. This is to
	// ensure that the cordoning phase will take a bit longer until we pull the
	// plug later on.
	drainBlocker, err := c.AppsV1().Deployments(metav1.NamespaceDefault).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "drain-blocker",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(s.ControllerCount)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test.k0sproject.io/app": "drain-blocker",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test.k0sproject.io/app": "drain-blocker",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: constant.KubePauseContainerImage + ":" + constant.KubePauseContainerImageVersion,
					}},
					Tolerations: []corev1.Toleration{
						// https://github.com/k0sproject/k0s/pull/5824
						{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule},
						constants.ControlPlaneToleration,
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
								TopologyKey: corev1.LabelHostname,
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"test.k0sproject.io/app": "drain-blocker",
									},
								},
							}},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	var pdbDeleted atomic.Bool
	_, err = c.PolicyV1().PodDisruptionBudgets(drainBlocker.Namespace).Create(ctx, &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name: drainBlocker.Name,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector:     drainBlocker.Spec.Selector,
			MinAvailable: ptr.To(intstr.FromInt32(*drainBlocker.Spec.Replicas)),
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.T().Log("Waiting for", drainBlocker.Name, "deployment")
	s.Require().NoError(common.WaitForDeployment(ctx, c, drainBlocker.Name, drainBlocker.Namespace))

	var wg sync.WaitGroup
	s.T().Cleanup(wg.Wait)

	kc, err := cf.GetK0sClient()
	s.Require().NoError(err)

	// TODO: Start those goroutines unconditionally in v1.36+.
	// These trigger bad behavior in old releases, which this test intentionally tries to uncover.
	// This behavior has been fixed in k0s v1.35+.
	if _, updatingFromOtherVersion := os.LookupEnv("K0S_UPDATE_FROM_PATH"); updatingFromOtherVersion {
		s.T().Log("Updating from another k0s version, skipping goroutines that trigger frequent reconcile events")
	} else {
		// Start some goroutines that will touch the ControlNode objects to trigger
		// a constant flow of reconcile events. This produces high concurrency load
		// on the Autopilot controllers, in order to test any races.
		s.T().Log("Starting goroutines to trigger frequent reconcile events")
		for idx := range s.ControllerCount {
			controlNodes, nodeName := kc.AutopilotV1beta2().ControlNodes(), s.ControllerNode(idx)
			wg.Add(1)
			go func() {
				defer wg.Done()
				wait.UntilWithContext(ctx, func(ctx context.Context) {
					_ = wait.ExponentialBackoffWithContext(ctx, retry.DefaultRetry, func(ctx context.Context) (bool, error) {
						cn, err := controlNodes.Get(ctx, nodeName, metav1.GetOptions{})
						if err == nil {
							if cn.Annotations == nil {
								cn.Annotations = map[string]string{}
							}
							cn.Annotations["test.k0sproject.io/touch"] = time.Now().Format("2006-01-02T15:04:05.000Z07:00")
							_, err = controlNodes.Update(ctx, cn, metav1.UpdateOptions{})
							if apierrors.IsConflict(err) {
								return false, nil
							}
						}
						if err != nil {
							s.T().Logf("Failed to touch %s: %v", nodeName, err)
						}

						return true, nil
					})
				}, 1*time.Second)
			}()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.T().Log("Monitoring ControlNodes to reach the", apsigcomm.Completed, "phase")
		phaseOrder := []string{
			apsigcomm.Downloading,
			apsigk0s.Cordoning,
			apsigk0s.ApplyingUpdate,
			apsigk0s.Restart,
			apsigk0s.UnCordoning,
			apsigcomm.Completed,
		}

		lastStatuses := make(map[string]*apsigv2.Status)
		err := watch.ControlNodes(kc.AutopilotV1beta2().ControlNodes()).
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

				status := signalData.Status
				if reflect.DeepEqual(lastStatus, status) {
					return false, nil
				} else if status == nil {
					return false, fmt.Errorf("signal status vanished from %s", node.Name)
				}

				lastStatuses[node.Name] = status

				order := slices.Index(phaseOrder, status.Status)
				if order < 0 {
					return false, fmt.Errorf("unexpected signal status %v for %s", status, node.Name)
				}

				if !pdbDeleted.Load() && order > slices.Index(phaseOrder, apsigk0s.Cordoning) {
					return false, fmt.Errorf("signal status of %s is %v, albeit the PDB hasn't been deleted yet", node.Name, status)
				}

				if lastStatus != nil {
					if status.Timestamp < lastStatus.Timestamp {
						return false, fmt.Errorf("signal status of %s went back in time, last observed signal status was %v: %v", node.Name, lastStatus, status)
					}
					if order < slices.Index(phaseOrder, lastStatus.Status) {
						return false, fmt.Errorf("signal status of %s went back in order, last observed signal status was %v: %v", node.Name, lastStatus, status)
					}
				}

				s.T().Log(node.Name, "signal status:", status)
				if len(lastStatuses) == s.ControllerCount {
					for _, status := range lastStatuses {
						if status.Status != apsigcomm.Completed {
							return false, nil
						}
					}
					return true, nil
				}

				return false, nil
			})

		if !s.NoError(err, "While monitoring ControlNodes to reach the %s phase", apsigcomm.Completed) {
			cancelTest(fmt.Errorf("failed to monitor ControlNodes to reach the %s phase", apsigcomm.Completed))
		}
	}()

	_, err = kc.AutopilotV1beta2().Plans().Create(ctx, &apv1beta2.Plan{
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
						"linux-amd64": apv1beta2.PlanResourceURL{URL: "http://localhost/dist/k0s-new"},
						"linux-arm64": apv1beta2.PlanResourceURL{URL: "http://localhost/dist/k0s-new"},
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
	s.T().Logf("Plan created")

	// After 30 secs, remove the PDB and allow the cordoning to proceed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.T().Log("Deleting", drainBlocker.Name, "PDB in 30 secs")
		select {
		case <-s.T().Context().Done():
		case <-time.After(30 * time.Second):
			pdbDeleted.Store(true) // Store it before actually deleting the PDB to prevent races.
			if s.NoError(c.PolicyV1().PodDisruptionBudgets(drainBlocker.Namespace).Delete(ctx, drainBlocker.Name, metav1.DeleteOptions{})) {
				s.T().Log("PDB deleted")
			} else {
				pdbDeleted.Store(false)
				cancelTest(fmt.Errorf("failed to delete %s PDB", drainBlocker.Name))
			}
		}
	}()

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	var plan *apv1beta2.Plan
	var resetTimer *time.Timer
	err = watch.Plans(kc.AutopilotV1beta2().Plans()).
		WithObjectName(apconst.AutopilotName).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(item *apv1beta2.Plan) (bool, error) {
			if resetTimer != nil {
				if resetTimer.Stop() {
					s.T().Log("Canceled reset to", appc.PlanSchedulable)
				}
			}

			switch item.Status.State {
			case appc.PlanSchedulable, appc.PlanSchedulableWait, "":
				return false, nil

			case appc.PlanCompleted:
				if !pdbDeleted.Load() {
					return false, errors.New("Plan execution completed too early")
				}
				plan = item
				return true, nil

			// TODO: Remove in v1.36+. This is a transitional helper case to allow
			// upgrade tests from older k0s versions to succeed.
			case "InconsistentTargets":
				if _, updatingFromOtherVersion := os.LookupEnv("K0S_UPDATE_FROM_PATH"); updatingFromOtherVersion {
					s.T().Log("Updating from another k0s version: InconsistentTargets encountered, resetting to", appc.PlanSchedulable, "after 3 seconds")
					toUpdate := item.DeepCopy()
					toUpdate.Status.State = appc.PlanSchedulable
					resetTimer = time.AfterFunc(3*time.Second, func() {
						_, err := kc.AutopilotV1beta2().Plans().UpdateStatus(ctx, toUpdate, metav1.UpdateOptions{})
						if err != nil {
							cancelTest(fmt.Errorf("failed to reset InconsistentTargets state: %w", err))
						}
					})
					return false, nil
				}
				fallthrough // Treat it as error otherwise

			default:
				return false, fmt.Errorf("unexpected plan state: %s", item.Status.State)
			}
		})
	if resetTimer != nil {
		if resetTimer.Stop() {
			s.T().Log("Canceled reset to", appc.PlanSchedulable)
		}
	}
	s.Require().NoError(err)

	if s.Len(plan.Status.Commands, 1) {
		cmd := plan.Status.Commands[0]

		s.Equal(appc.PlanCompleted, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.NotNil(cmd.K0sUpdate.Controllers)
		s.Empty(cmd.K0sUpdate.Workers)

		for _, node := range cmd.K0sUpdate.Controllers {
			s.Equal(appc.SignalCompleted, node.State)
		}
	}

	for idx := range s.BootlooseSuite.ControllerCount {
		nodeName, require := s.ControllerNode(idx), s.Require()
		require.NoError(s.WaitForNodeReady(nodeName, c))
		// Wait till we see kubelet reporting the expected version.
		// This is only bullet proof if upgrading to _another_ Kubernetes version.
		err := watch.Nodes(c.CoreV1().Nodes()).
			WithObjectName(nodeName).
			WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
			Until(ctx, func(node *corev1.Node) (bool, error) {
				return strings.Contains(node.Status.NodeInfo.KubeletVersion, fmt.Sprintf("v%s.", constant.KubernetesMajorMinorVersion)), nil
			})
		require.NoError(err)
	}
}

func TestControllerWorkerSuite(t *testing.T) {
	suite.Run(t, &controllerworkerSuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
			WithLB:          true,
		},
	})
}
