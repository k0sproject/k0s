// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type CPLBReconcilerSuite struct {
	suite.Suite
}

func successfulRestResult(ctx context.Context) restResult {
	return restResultMock{success: true}
}

func failingRestResult(ctx context.Context) restResult {
	return restResultMock{}
}

type restResultMock struct {
	success bool
}

func (r restResultMock) Error() error {
	if r.success {
		return nil
	}
	return context.DeadlineExceeded
}

func TestHealthCheckerRunHealthCheck(t *testing.T) {
	updateCh := make(chan struct{})

	hc := &healthChecker{
		log:           logrus.WithField("component", "cplb-healthchecker-test"),
		healthy:       false,
		healthCheckFn: successfulRestResult,
		updateCh:      updateCh,
	}
	c := make(chan time.Time)

	go hc.runHealthCheck(t.Context(), "192.168.1.1", c, func() {})
	require.False(t, hc.healthy, "Expected the healthChecker to be unhealthy before the first tick")

	c <- time.Now()
	<-updateCh
	require.True(t, hc.healthy, "Expected the healthChecker to be healthy after a successful health check")

	hc.healthCheckFn = failingRestResult
	c <- time.Now()
	<-updateCh
	require.False(t, hc.healthy, "Expected the healthChecker to be unhealthy after a failed health check")
}

func TestNewHealthChecker(t *testing.T) {
	t.Run("Invalid restConfig should always return healthy", func(t *testing.T) {
		updateCh := make(chan struct{}, 1)
		reconciler := &CPLBReconciler{
			apiPort:  6443,
			log:      logrus.WithField("component", "cplb-reconciler-test"),
			updateCh: updateCh,
		}
		hc := reconciler.newHealthChecker(t.Context(), nil, "192.168.1.1", nil)
		<-updateCh
		require.True(t, hc.healthy)
	})
}

func TestMaybeUpdateIPs(t *testing.T) {
	ch := make(chan struct{}, 1)
	ipA := "192.168.1.1"
	ipB := "192.168.1.2"
	reconciler := &CPLBReconciler{
		apiPort:        6443,
		log:            logrus.WithField("component", "cplb-reconciler"),
		updateCh:       ch,
		healthCheckers: make(map[string]*healthChecker),
	}
	reconciler.maybeUpdateIPs(t.Context(), getEndpointsWithIPs([]string{ipA}), nil)

	select {
	case <-ch:
		require.Equal(t, []string{ipA}, reconciler.GetIPs(), "Expected the addresses to be updated")
	default:
		require.FailNow(t, "Expected an update signal on the updateCh channel")
	}

	reconciler.maybeUpdateIPs(t.Context(), getEndpointsWithIPs([]string{ipA}), nil)
	select {
	case <-ch:
		require.FailNow(t, "Unexpected an update signal on the updateCh channel")
	default:
		require.Equal(t, []string{ipA}, reconciler.GetIPs(), "Expected the addresses to be updated")
	}

	reconciler.maybeUpdateIPs(t.Context(), getEndpointsWithIPs([]string{ipA, ipB}), nil)
	select {
	case <-ch:
		ips := reconciler.GetIPs()
		slices.Sort(ips)
		require.Equal(t, []string{ipA, ipB}, ips, "Expected the addresses to be updated")
	default:
		require.FailNow(t, "Expected an update signal on the updateCh channel")
	}

	reconciler.maybeUpdateIPs(t.Context(), getEndpointsWithIPs([]string{ipA}), nil)
	select {
	case <-ch:
		require.Equal(t, []string{ipA}, reconciler.GetIPs(), "Expected the addresses to be updated")
	default:
		require.FailNow(t, "Expected an update signal on the updateCh channel")
	}

	reconciler.maybeUpdateIPs(t.Context(), getEndpointsWithIPs([]string{}), nil)
	select {
	case <-ch:
		require.Empty(t, reconciler.GetIPs(), "Expected the addresses to be updated")
	default:
		require.FailNow(t, "Expected an update signal on the updateCh channel")
	}
}

func getEndpointsWithIPs(ips []string) *corev1.Endpoints {
	addresses := make([]corev1.EndpointAddress, len(ips))
	for i, ip := range ips {
		addresses[i] = corev1.EndpointAddress{IP: ip}
	}

	endpoints := &corev1.Endpoints{
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: addresses,
			},
		},
	}
	return endpoints
}
