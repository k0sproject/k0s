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

package k0s

import (
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// restartEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func restartEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return nil
}

// registerRestart registers the 'restart' controller to the controller-runtime manager.
//
// This controller is only interested in changes to signal nodes where its signaling
// status is marked as `Restart`
func registerRestart(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	return nil
}
