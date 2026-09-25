// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"sigs.k8s.io/yaml"
)

// K0SControlAPI implements the k0s control API component
type K0SControlAPI struct {
	RuntimeConfig *config.RuntimeConfig

	supervisor *supervisor.Supervisor
	uid        int
}

var _ manager.Component = (*K0SControlAPI)(nil)

// Init does currently nothing
func (m *K0SControlAPI) Init(_ context.Context) error {
	// We need to create a serving cert for the api
	var err error
	m.uid, err = users.LookupUID(constant.ApiserverUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.ApiserverUser, err)
		m.uid = users.RootUID
		logrus.WithError(err).Warn("Running k0s control API as root")
	}
	return nil
}

// Run runs k0s control api as separate process
func (m *K0SControlAPI) Start(ctx context.Context) error {
	selfExe, err := os.Executable()
	if err != nil {
		return err
	}

	runtimeConfig, err := yaml.Marshal(m.RuntimeConfig)
	if err != nil {
		return err
	}

	m.supervisor = &supervisor.Supervisor{
		Name:    "k0s-control-api",
		BinPath: selfExe,
		RunDir:  m.RuntimeConfig.Spec.K0sVars.RunDir,
		DataDir: m.RuntimeConfig.Spec.K0sVars.DataDir,
		Args:    []string{"api"},
		Stdin:   func() io.Reader { return bytes.NewReader(runtimeConfig) },
		UID:     m.uid,
	}

	return m.supervisor.Supervise(ctx)
}

// Stop stops k0s api
func (m *K0SControlAPI) Stop() error {
	if m.supervisor != nil {
		return m.supervisor.Stop()
	}
	return nil
}
