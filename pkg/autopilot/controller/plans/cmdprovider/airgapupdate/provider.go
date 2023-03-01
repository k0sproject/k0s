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

package airgapupdate

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appkd "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/discovery"
	appku "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/cmdprovider/k0supdate/utils"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	commandID = "AirgapUpdate"
)

type airgapupdate struct {
	logger                *logrus.Entry
	client                crcli.Client
	controllerDelegateMap apdel.ControllerDelegateMap
	excludedFromPlans     map[string]struct{}
	cf                    kubernetes.ClientFactoryInterface
}

var _ appc.PlanCommandProvider = (*airgapupdate)(nil)

func NewAirgapUpdatePlanCommandProvider(logger *logrus.Entry, client crcli.Client, dm apdel.ControllerDelegateMap, cf kubernetes.ClientFactoryInterface, excludeFromPlans []string) appc.PlanCommandProvider {
	excludedFromPlans := make(map[string]struct{})
	for _, excluded := range excludeFromPlans {
		excludedFromPlans[excluded] = struct{}{}
	}

	return &airgapupdate{
		logger:                logger.WithField("command", "airgapupdate"),
		client:                client,
		controllerDelegateMap: dm,
		cf:                    cf,
		excludedFromPlans:     excludedFromPlans,
	}
}

func (aup *airgapupdate) CommandID() string {
	return commandID
}

// populateWorkerStatus is a specialization of `DiscoverNodes` for working
// with `v1.Node` signal node objects.
func populateWorkerStatus(ctx context.Context, client crcli.Client, update apv1beta2.PlanCommandAirgapUpdate, dm apdel.ControllerDelegateMap) ([]apv1beta2.PlanCommandTargetStatus, bool) {
	return appkd.DiscoverNodes(ctx, client, &update.Workers, dm["worker"], func(name string) (bool, *apv1beta2.PlanCommandTargetStateType) {
		return appku.ObjectExistsWithPlatform(ctx, client, name, &v1.Node{}, update.Platforms)
	})
}
