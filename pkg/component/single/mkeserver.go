package single

import (
	"os"

	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

// MkeServer implement the component interface to run mke server
type MkeServer struct {
	Debug      bool
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (m *MkeServer) Init() error {
	return nil
}

// Run runs mke server
func (m *MkeServer) Run() error {
	logrus.Info("Starting mke server")
	m.supervisor = supervisor.Supervisor{
		Name:    "mke server",
		BinPath: os.Args[0],
	}

	if m.Debug {
		m.supervisor.Args = append(m.supervisor.Args, "--debug")
	}
	m.supervisor.Args = append(m.supervisor.Args, "server")

	m.supervisor.Supervise()
	return nil
}

// Stop stops mke server
func (m *MkeServer) Stop() error {
	return m.supervisor.Stop()
}
