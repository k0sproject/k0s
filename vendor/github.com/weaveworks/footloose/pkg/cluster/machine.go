package cluster

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/footloose/pkg/config"
	"github.com/weaveworks/footloose/pkg/docker"
	"github.com/weaveworks/footloose/pkg/exec"
	"github.com/weaveworks/footloose/pkg/ignite"
)

// Machine is a single machine.
type Machine struct {
	spec *config.Machine

	// container name.
	name string
	// container hostname.
	hostname string
	// container ip.
	ip string

	runtimeNetworks []*RuntimeNetwork
	// Fields that are cached from the docker daemon.

	ports map[int]int
	// maps containerPort -> hostPort.
}

// ContainerName is the name of the running container corresponding to this
// Machine.
func (m *Machine) ContainerName() string {
	if m.IsIgnite() {
		filter := fmt.Sprintf(`label=ignite.name=%s`, m.name)
		cid, err := exec.ExecuteCommand("docker", "ps", "-q", "-f", filter)
		if err != nil || len(cid) == 0 {
			return m.name
		}
		return cid
	}
	return m.name
}

// Hostname is the machine hostname.
func (m *Machine) Hostname() string {
	return m.hostname
}

// IsCreated returns if a machine is has been created. A created machine could
// either be running or stopped.
func (m *Machine) IsCreated() bool {
	if m.IsIgnite() {
		return ignite.IsCreated(m.name)
	}

	res, _ := docker.Inspect(m.name, "{{.Name}}")
	if len(res) > 0 && len(res[0]) > 0 {
		return true
	}
	return false
}

// IsStarted returns if a machine is currently started or not.
func (m *Machine) IsStarted() bool {
	if m.IsIgnite() {
		return ignite.IsStarted(m.name)
	}

	res, _ := docker.Inspect(m.name, "{{.State.Running}}")
	parsed, _ := strconv.ParseBool(strings.Trim(res[0], `'`))
	return parsed
}

// HostPort returns the host port corresponding to the given container port.
func (m *Machine) HostPort(containerPort int) (int, error) {
	// Use the cached version first
	if hostPort, ok := m.ports[containerPort]; ok {
		return hostPort, nil
	}

	var hostPort int

	// Handle Ignite VMs
	if m.IsIgnite() {
		// Retrieve the machine details
		vm, err := ignite.PopulateMachineDetails(m.name)
		if err != nil {
			return -1, errors.Wrap(err, "failed to populate VM details")
		}

		// Find the host port for the given VM port
		var found = false
		for _, p := range vm.Spec.Network.Ports {
			if int(p.VMPort) == containerPort {
				hostPort = int(p.HostPort)
				found = true
				break
			}
		}

		if !found {
			return -1, fmt.Errorf("invalid VM port queried: %d", containerPort)
		}
	} else {
		// retrieve the specific port mapping using docker inspect
		lines, err := docker.Inspect(m.ContainerName(), fmt.Sprintf("{{(index (index .NetworkSettings.Ports \"%d/tcp\") 0).HostPort}}", containerPort))
		if err != nil {
			return -1, errors.Wrapf(err, "hostport: failed to inspect container: %v", lines)
		}
		if len(lines) != 1 {
			return -1, errors.Errorf("hostport: should only be one line, got %d lines", len(lines))
		}

		port := strings.Replace(lines[0], "'", "", -1)
		if hostPort, err = strconv.Atoi(port); err != nil {
			return -1, errors.Wrap(err, "hostport: failed to parse string to int")
		}
	}

	if m.ports == nil {
		m.ports = make(map[int]int)
	}

	// Cache the result
	m.ports[containerPort] = hostPort
	return hostPort, nil
}

func (m *Machine) networks() ([]*RuntimeNetwork, error) {
	if len(m.runtimeNetworks) != 0 {
		return m.runtimeNetworks, nil
	}

	var networks map[string]*network.EndpointSettings
	if err := docker.InspectObject(m.name, ".NetworkSettings.Networks", &networks); err != nil {
		return nil, err
	}
	m.runtimeNetworks = NewRuntimeNetworks(networks)
	return m.runtimeNetworks, nil
}

func (m *Machine) igniteStatus(s *MachineStatus) error {
	vm, err := ignite.PopulateMachineDetails(m.name)
	if err != nil {
		return err
	}

	// Set Ports
	var ports []port
	for _, p := range vm.Spec.Network.Ports {
		ports = append(ports, port{
			Host:  int(p.HostPort),
			Guest: int(p.VMPort),
		})
	}
	s.Ports = ports
	if vm.Status.IpAddresses != nil && len(vm.Status.IpAddresses) > 0 {
		m.ip = vm.Status.IpAddresses[0]
	}

	s.RuntimeNetworks = NewIgniteRuntimeNetwork(&vm.Status)

	return nil
}

func (m *Machine) dockerStatus(s *MachineStatus) error {
	var ports []port
	if m.IsCreated() {
		for _, v := range m.spec.PortMappings {
			hPort, err := m.HostPort(int(v.ContainerPort))
			if err != nil {
				hPort = 0
			}
			p := port{
				Host:  hPort,
				Guest: int(v.ContainerPort),
			}
			ports = append(ports, p)
		}
	}
	if len(ports) < 1 {
		for _, p := range m.spec.PortMappings {
			ports = append(ports, port{Host: 0, Guest: int(p.ContainerPort)})
		}
	}
	s.Ports = ports

	s.RuntimeNetworks, _ = m.networks()

	return nil
}

// Status returns the machine status.
func (m *Machine) Status() *MachineStatus {
	s := MachineStatus{}
	s.Container = m.ContainerName()
	s.Image = m.spec.Image
	s.Command = m.spec.Cmd
	s.Spec = m.spec
	s.Hostname = m.Hostname()
	s.IP = m.ip
	state := NotCreated

	if m.IsCreated() {
		state = Stopped
		if m.IsStarted() {
			state = Running
		}
	}
	s.State = state

	if m.IsIgnite() {
		_ = m.igniteStatus(&s)
	} else {
		_ = m.dockerStatus(&s)
	}

	return &s
}

// Only check for Ignite prerequisites once
var igniteChecked bool

// IsIgnite returns if the backend is Ignite
func (m *Machine) IsIgnite() (b bool) {
	b = m.spec.Backend == ignite.BackendName

	if !igniteChecked && b {
		if syscall.Getuid() != 0 {
			log.Fatalf("Footloose needs to run as root to use the %q backend", ignite.BackendName)
		}

		ignite.CheckVersion()
		igniteChecked = true
	}

	return
}
