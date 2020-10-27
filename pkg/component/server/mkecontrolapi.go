/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"fmt"
	"os"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/supervisor"
)

// MkeControlAPI implements the mke control API component
type MkeControlAPI struct {
	ConfigPath    string
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
}

// Init does currently nothing
func (m *MkeControlAPI) Init() error {
	// We need to create a serving cert for the api
	return nil
}

// Run runs mke control api as separate process
func (m *MkeControlAPI) Run() error {
	// TODO: Make the api process to use some other user
	m.supervisor = supervisor.Supervisor{
		Name:    "mke-control-api",
		BinPath: os.Args[0],
		Args: []string{
			"api",
			fmt.Sprintf("--config=%s", m.ConfigPath),
		},
	}

	m.supervisor.Supervise()
	return nil
}

// Stop stops mke api
func (m *MkeControlAPI) Stop() error {
	return m.supervisor.Stop()
}

// Health-check interface
func (m *MkeControlAPI) Healthy() error { return nil }
