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

	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

type SignalDataPredicate func(signalData apsigv2.SignalData) bool

// SignalDataPredicateAdapter defines logical operations that can be performed
// on `SignalDataPredicate`s
type SignalDataPredicateAdapter interface {
	And(preds ...SignalDataPredicate) crpred.Predicate
}

type signalDataPredicateAdapter struct {
	handler ErrorHandler
}

// NewSignalDataPredicateAdapter creates a new `SignalDataPredicateAdapter` with
// an error handler used by the operations.
func NewSignalDataPredicateAdapter(handler ErrorHandler) SignalDataPredicateAdapter {
	return &signalDataPredicateAdapter{
		handler: handler,
	}
}

// And performs an AND operation across the provided `SignalDataPredicate`s, stopping
// at the first unsuccessful predicate.
func (sdp signalDataPredicateAdapter) And(preds ...SignalDataPredicate) crpred.Predicate {
	return crpred.NewPredicateFuncs(func(obj crcli.Object) bool {
		var signalData apsigv2.SignalData

		if err := signalData.Unmarshal(obj.GetAnnotations()); err != nil {
			return sdp.handler(err)
		}

		for _, pred := range preds {
			if !pred(signalData) {
				return false
			}
		}

		return true
	})
}
