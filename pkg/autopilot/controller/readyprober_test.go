//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8stesting "k8s.io/client-go/testing"
)

func TestReadyProberTargetResolutionFailed(t *testing.T) {
	cf := testutil.NewFakeClientFactory()
	targets := []autopilotv1beta2.PlanCommandTargetStatus{{Name: "controller0"}}

	prober := newTestProber(t, 0, cf)
	err := prober.probeTargets(t.Context(), targets)

	assert.ErrorIs(t, err, errReadyProbeTargetResolutionFailed)
	assert.NotErrorIs(t, err, errUnsuccessfulReadyProbe)
	assert.ErrorContains(t, err, "while probing controller0: target resolution failed: no such ControlNode: controlnodes.autopilot.k0sproject.io \"controller0\" not found")
}

func TestReadyProberMissingInternalIP(t *testing.T) {
	cf := testutil.NewFakeClientFactory(newControlNode("controller0", ""))
	targets := []autopilotv1beta2.PlanCommandTargetStatus{{Name: "controller0"}}

	prober := newTestProber(t, 0, cf)
	err := prober.probeTargets(t.Context(), targets)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, errReadyProbeTargetResolutionFailed)
	assert.NotErrorIs(t, err, errUnsuccessfulReadyProbe)
	assert.ErrorContains(t, err, "while probing controller0: no internal IP address found for ControlNode")
}

func TestReadyProberUnsuccessfulProbe(t *testing.T) {
	var numRequests atomic.Uint32
	hostPort := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("nope"))
		assert.NoError(t, err)
	}))
	cf := testutil.NewFakeClientFactory(newControlNode("controller0", hostPort.Host()))
	targets := []autopilotv1beta2.PlanCommandTargetStatus{{Name: "controller0"}}

	prober := newTestProber(t, int(hostPort.Port()), cf)
	err := prober.probeTargets(t.Context(), targets)

	assert.Positive(t, numRequests.Load())
	assert.ErrorIs(t, err, errUnsuccessfulReadyProbe)
	assert.NotErrorIs(t, err, errReadyProbeTargetResolutionFailed)
	assert.ErrorContains(t, err, "while probing controller0: ready probe didn't indicate success: failure: HTTP probe failed with statuscode: 500")
}

func TestReadyProberSuccess(t *testing.T) {
	var numRequests atomic.Uint32
	hostPort := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	cf := testutil.NewFakeClientFactory(newControlNode("controller0", hostPort.Host()))
	targets := []autopilotv1beta2.PlanCommandTargetStatus{{Name: "controller0"}}

	prober := newTestProber(t, int(hostPort.Port()), cf)
	err := prober.probeTargets(t.Context(), targets)

	assert.Positive(t, numRequests.Load())
	assert.NoError(t, err)
}

func TestReadyProberContextCanceled(t *testing.T) {
	hostPort := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cf := testutil.NewFakeClientFactory(newControlNode("controller0", hostPort.Host()))
	targets := []autopilotv1beta2.PlanCommandTargetStatus{{Name: "controller0"}}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	prober := newTestProber(t, int(hostPort.Port()), cf)
	err := prober.probeTargets(canceled, targets)

	assert.NotErrorIs(t, err, errReadyProbeTargetResolutionFailed)
	assert.ErrorContains(t, err, "while probing controller0")
	assert.ErrorContains(t, err, "context canceled")
}

// Probe two controllers, in a way that one of them will fail its readiness
// probe, and the other won't exist, ensuring that the goroutine that fails the
// readiness probe will terminate first. This will ensure that a target
// resolution error will effectively override a general readiness probe error.
func TestReadyProberPrioritizesTargetResolutionFailure(t *testing.T) {
	var numRequests atomic.Uint32
	hostPort := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	synctest.Test(t, func(t *testing.T) {
		cf := testutil.NewFakeClientFactory(
			newControlNode("controller1", hostPort.Host()),
			newControlNode("controller2", hostPort.Host()),
		)

		var existingController atomic.Pointer[string]
		cf.K0sClient.PrependReactor("get", "controlnodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			controller := action.(k8stesting.GetAction).GetName()
			// Do nothing if it's the "existing" controller.
			if existingController.CompareAndSwap(nil, &controller) || controller == *existingController.Load() {
				return false, nil, nil
			}

			// This is the "missing" controller.
			// Ensure that all other goroutines have finished.
			synctest.Wait()

			// Now it's guaranteed that the goroutine that hit the existing
			// controller has finished and recorded its error. Now go ahead
			// and emulate the absence of this controller.
			assert.Positive(t, numRequests.Load(), "Expected to see an HTTP request for the existing controller")
			return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), controller)
		})

		targets := []autopilotv1beta2.PlanCommandTargetStatus{
			{Name: "controller1"},
			{Name: "controller2"},
		}

		prober := newTestProber(t, int(hostPort.Port()), cf)
		err := prober.probeTargets(t.Context(), targets)

		assert.Positive(t, numRequests.Load())
		require.NotErrorIs(t, err, errUnsuccessfulReadyProbe)
		require.ErrorIs(t, err, errReadyProbeTargetResolutionFailed)
		assert.NotContains(t, err.Error(), *existingController.Load())
	})
}

func newTestProber(t *testing.T, port int, cf *testutil.FakeClientFactory) *readyProber {
	logger := logrus.New().WithField("test", t.Name())
	prober := newReadyProber(logger, cf.K0sClient, &tls.Config{InsecureSkipVerify: true}, port, 2*time.Second)
	return prober
}

func newControlNode(name, internalIP string) *autopilotv1beta2.ControlNode {
	controlNode := &autopilotv1beta2.ControlNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	if internalIP != "" {
		controlNode.Status.Addresses = []corev1.NodeAddress{{
			Type:    corev1.NodeInternalIP,
			Address: internalIP,
		}}
	}

	return controlNode
}

func newTestServer(t *testing.T, handler http.Handler) *k0snet.HostPort {
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	hostPort, err := k0snet.ParseHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	return hostPort
}
