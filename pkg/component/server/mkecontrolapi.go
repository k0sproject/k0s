package server

import (
	"os"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/supervisor"
)

// MkeControlApi
type MkeControlApi struct {
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
}

func (m *MkeControlApi) Init() error {
	// We need to create a serving cert for the api

	return nil
}

// Run runs mke control api as separate process
func (m *MkeControlApi) Run() error {
	// TODO: Make the api process to use some other user
	m.supervisor = supervisor.Supervisor{
		Name:    "mke control api",
		BinPath: os.Args[0],
		Args: []string{
			"api",
		},
	}

	m.supervisor.Supervise()
	return nil
}

// Stop stops mke api
func (m *MkeControlApi) Stop() error {
	return m.supervisor.Stop()
}
