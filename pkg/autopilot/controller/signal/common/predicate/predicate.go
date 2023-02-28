// Copyright 2021 k0s authors
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

package predicate

import (
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ErrorHandler func(err error) bool

// DefaultErrorHandler is a predicate error handler that simply logs.
func DefaultErrorHandler(logger *logrus.Entry, name string) ErrorHandler {
	return func(err error) bool {
		return false
	}
}

// SignalNamePredicate creates a controller-runtime predicate that
// ensures that the object in question is a signal node that has a name
// that matches the provided hostname.
func SignalNamePredicate(hostname string) crpred.Predicate {
	return crpred.NewPredicateFuncs(func(obj crcli.Object) bool {
		return obj.GetName() == hostname
	})
}

// SignalDataStatusPredicate creates a predicate that ensures that SignalData
// status matches the provided value.
func SignalDataStatusPredicate(status string) SignalDataPredicate {
	return func(signalData apsigv2.SignalData) bool {
		return signalData.Status != nil && signalData.Status.Status == status
	}
}

// SignalDataNoStatusPredicate creates a predicate that ensures that no status is
// present on the provided SignalData.
func SignalDataNoStatusPredicate() SignalDataPredicate {
	return func(signalData apsigv2.SignalData) bool {
		return signalData.Status == nil
	}
}
