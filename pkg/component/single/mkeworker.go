package single

import (
	"os"
	"os/exec"
	"time"

	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
)

// MkeWorker implement the component interface to run mke server
type MkeWorker struct {
	Debug      bool
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (m *MkeWorker) Init() error {
	return nil
}

// Run runs mke server
func (m *MkeWorker) Run() error {
	logrus.Info("Starting mke worker")
	m.supervisor = supervisor.Supervisor{
		Name:    "mke worker",
		BinPath: os.Args[0],
	}

	if m.Debug {
		m.supervisor.Args = append(m.supervisor.Args, "--debug")
	}
	m.supervisor.Args = append(m.supervisor.Args, "worker", "--server", "https://localhost:6443")

	if !util.FileExists("/var/lib/mke/kubelet.conf") {
		// wait for server to start up
		// TODO: monitor needed config file instead of sleep
		time.Sleep(4 * time.Second)
		// create token
		token, err := exec.Command(os.Args[0], "token", "create").Output()
		if err != nil {
			logrus.Error("failed to execute mke token create: ", err)
			return err
		}
		m.supervisor.Args = append(m.supervisor.Args, string(token))
	}
	m.supervisor.Supervise()

	return nil
}

// Stop stops mke worker
func (m *MkeWorker) Stop() error {
	return m.supervisor.Stop()
}
