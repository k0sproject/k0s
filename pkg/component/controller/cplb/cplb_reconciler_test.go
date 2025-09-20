// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/ptr"
)

type CPLBReconcilerSuite struct {
	suite.Suite
}

func (s *CPLBReconcilerSuite) TestMaybeUpdateIPs() {
	ch := make(chan struct{}, 1)
	var updateCh <-chan struct{} = ch
	reconciler := &CPLBReconciler{
		addresses: []string{},
		updateCh:  ch,
		log:       logrus.WithField("component", "cplb-reconciler-test"),
	}

	endpoints := &discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.1.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				Addresses: []string{"192.168.1.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	// test the addresses change when the endpoints change and the channel is notified
	reconciler.maybeUpdateIPs(endpoints)
	select {
	case <-updateCh:
		s.Require().Equal([]string{"192.168.1.1", "192.168.1.2"}, reconciler.GetIPs(), "Expected the addresses to be updated")
	default:
		s.FailNow("Expected an update signal on the updateCh channel")
	}

	// test the addresses don't change when the endpoints don't change and the channel isn't notified.
	reconciler.maybeUpdateIPs(endpoints)
	select {
	case <-updateCh:
		s.FailNow("Expected no update signal on the updateCh channel")
	default:
		s.Require().Equal([]string{"192.168.1.1", "192.168.1.2"}, reconciler.GetIPs(), "Unexpected addresses change")
	}

	// test the addresses change when the endpoints change and the channel is notified when the addresses are empty
	endpoints.Endpoints = []discoveryv1.Endpoint{}

	reconciler.maybeUpdateIPs(endpoints)
	select {
	case <-updateCh:
		s.Require().Equal([]string{}, reconciler.GetIPs(), "Expected the addresses to be updated")
	default:
		s.FailNow("Expected an update signal on the updateCh channel")
	}
}

func TestCPLBReconcilerSuite(t *testing.T) {
	cplbReconcilerSuite := &CPLBReconcilerSuite{}

	suite.Run(t, cplbReconcilerSuite)
}
