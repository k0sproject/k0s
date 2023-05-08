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

package signal

import (
	"context"
	"fmt"

	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigag "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/airgap"
	apsigk0s "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/k0s"

	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterControllers registers all of the autopilot controllers used by both controller
// and worker modes.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, k0sDataDir, clusterID string) error {
	if err := apsigk0s.RegisterControllers(ctx, logger, mgr, delegate, clusterID); err != nil {
		return fmt.Errorf("unable to register k0s controllers: %w", err)
	}

	if err := apsigag.RegisterControllers(ctx, logger, mgr, delegate, k0sDataDir); err != nil {
		return fmt.Errorf("unable to register airgap controllers: %w", err)
	}

	return nil
}
