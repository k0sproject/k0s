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

package k0supdate

import (
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/sirupsen/logrus"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	commandID = "K0sUpdate"
)

type k0supdate struct {
	logger                *logrus.Entry
	client                crcli.Client
	controllerDelegateMap apdel.ControllerDelegateMap
	excludedFromPlans     map[string]struct{}
	cf                    kubernetes.ClientFactoryInterface
}

var _ appc.PlanCommandProvider = (*k0supdate)(nil)

// NewK0sUpdatePlanCommandProvider builds a `PlanCommandProvider` for the
// `K0sUpdate` command.
func NewK0sUpdatePlanCommandProvider(logger *logrus.Entry, client crcli.Client, dm apdel.ControllerDelegateMap, cf kubernetes.ClientFactoryInterface, excludeFromPlans []string) appc.PlanCommandProvider {
	excludedFromPlans := make(map[string]struct{})
	for _, excluded := range excludeFromPlans {
		excludedFromPlans[excluded] = struct{}{}
	}

	return &k0supdate{
		logger:                logger.WithField("command", "k0supdate"),
		client:                client,
		controllerDelegateMap: dm,
		cf:                    cf,
		excludedFromPlans:     excludedFromPlans,
	}
}

// CommandID is the identifier of the command which needs to match the field name of the
// command in `PlanCommand`.
func (kp *k0supdate) CommandID() string {
	return commandID
}
