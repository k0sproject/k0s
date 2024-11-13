package service

import (
	"errors"
	"fmt"

	"github.com/kardianos/service"
)

// Status represents the status of a system service
type Status string

const (
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusUnknown      Status = "unknown"
	StatusNotInstalled Status = "not installed"

	k0sServicePrefix = "k0s"
	k0sDescription   = "k0s - Zero Friction Kubernetes"
)

var ErrK0sNotInstalled = errors.New("k0s has not been installed as a system service")

type Service struct {
	svc service.Service
}

// dummy implementation for kardianos.Interface
type program struct{}

func (p *program) Start(service.Service) error {
	// Start should not block. Do the actual work async.
	return nil
}

func (p *program) Stop(service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

// NewService creates a new system service instance
func NewService(cfg *Config) (*Service, error) {
	kardianosCfg := &service.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Arguments:   cfg.Arguments,
		Option:      cfg.Option,
	}

	kardSvc, err := service.New(&program{}, kardianosCfg)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}
	return &Service{svc: kardSvc}, nil
}

// K0sConfig returns a Config for a k0s system service
func K0sConfig(role string) *Config {
	var k0sDisplayName, k0sServiceName string

	if role == "controller" || role == "worker" {
		k0sDisplayName = k0sServicePrefix + " " + role
		k0sServiceName = k0sServicePrefix + role
	}
	return &Config{
		Name:        k0sServiceName,
		DisplayName: k0sDisplayName,
		Description: k0sDescription,
	}
}

func InstalledK0sService() (*Service, error) {
	for _, role := range []string{"controller", "worker"} {
		c := K0sConfig(role)
		s, err := NewService(c)
		if err != nil {
			return nil, err
		}
		status, err := s.Status()
		if err != nil {
			return nil, err
		}

		if status != StatusNotInstalled {
			return s, nil
		}
	}
	return nil, ErrK0sNotInstalled
}

// Start the system service
func (s *Service) Start() error {
	if err := s.svc.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

// Stop the system service
func (s *Service) Stop() error {
	if err := s.svc.Stop(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	return nil
}

// Status of the system service or an error if the status could not be determined
func (s *Service) Status() (Status, error) {
	status, err := s.svc.Status()
	if err != nil {
		if errors.Is(err, service.ErrNotInstalled) {
			return StatusNotInstalled, nil
		}
		return StatusUnknown, fmt.Errorf("get service status: %w", err)
	}

	// Map kardianos status codes to our defined constants
	switch status {
	case service.StatusRunning:
		return StatusRunning, nil
	case service.StatusStopped:
		return StatusStopped, nil
	default:
		return StatusUnknown, nil
	}
}

// Uninstall the system service
func (s *Service) Uninstall() error {
	return s.svc.Uninstall()
}

// Install the system service
func (s *Service) Install() error {
	return s.svc.Install()
}

// Platform returns the init system used by the host
func (s *Service) Platform() string {
	return s.svc.Platform()
}
