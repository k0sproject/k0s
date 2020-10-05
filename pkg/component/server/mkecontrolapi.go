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
