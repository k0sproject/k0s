/*
Copyright 2020 k0s authors

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
	"context"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// K0SControlAPI implements the k0s control API component
type K0SControlAPI struct {
	ConfigPath string
	K0sVars    *config.CfgVars
	supervisor supervisor.Supervisor
}

var _ manager.Component = (*K0SControlAPI)(nil)

// Init does currently nothing
func (m *K0SControlAPI) Init(_ context.Context) error {
	// We need to create a serving cert for the api
	return nil
}

// Run runs k0s control api as separate process
func (m *K0SControlAPI) Start(_ context.Context) error {
	// TODO: Make the api process to use some other user

	selfExe, err := os.Executable()
	if err != nil {
		return err
	}
	m.supervisor = supervisor.Supervisor{
		Name:    "k0s-control-api",
		BinPath: selfExe,
		RunDir:  m.K0sVars.RunDir,
		DataDir: m.K0sVars.DataDir,
		Args: []string{
			"api",
			fmt.Sprintf("--data-dir=%s", m.K0sVars.DataDir),
		},
	}

	return m.supervisor.Supervise()
}

// Stop stops k0s api
func (m *K0SControlAPI) Stop() error {
	return m.supervisor.Stop()
}
