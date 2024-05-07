/*
Copyright 2024 k0s authors

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

package controller

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
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

	endpoints := &corev1.Endpoints{
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "192.168.1.1"},
					{IP: "192.168.1.2"},
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
	endpoints.Subsets[0].Addresses = []corev1.EndpointAddress{}
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
