package install

import (
	"fmt"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

const (
	k0sServiceName = "k0s"
	k0sDescription = "k0s - Zero Friction Kubernetes"
)

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

// EnsureService installs the k0s service, per the given arguments, and the detected platform
func EnsureService(args []string) error {
	var deps []string
	var k0sDisplayName string

	prg := &program{}

	for _, v := range args {
		if v == "server" {
			k0sDisplayName = "k0s server"
		} else {
			k0sDisplayName = "k0s worker"
		}

	}

	// initial svc config
	svcConfig := &service.Config{
		Name:        k0sServiceName,
		DisplayName: k0sDisplayName,
		Description: k0sDescription,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	// fetch service type
	svcType := s.Platform()
	switch svcType {
	case "linux-openrc":
		deps = []string{"need net", "use dns", "after firewall"}
	case "linux-systemd":
		deps = []string{"After=network.target"}
	default:
	}

	svcConfig.Dependencies = deps
	svcConfig.Arguments = args

	logrus.Info("Installing k0s service")
	err = s.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %v", err)
	}
	return nil
}
