package ignite

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/weaveworks/footloose/pkg/config"
	"github.com/weaveworks/footloose/pkg/exec"
)

const (
	BackendName = "ignite"
)

// This offset is incremented for each port so we avoid
// duplicate port bindings (and hopefully port collisions)
var portOffset uint16

// Create creates an Ignite VM using "ignite run", it doesn't return a container ID
func Create(name string, spec *config.Machine, pubKeyPath string) (id string, err error) {
	runArgs := []string{
		"run",
		spec.Image,
		fmt.Sprintf("--name=%s", name),
		fmt.Sprintf("--cpus=%d", spec.IgniteConfig().CPUs),
		fmt.Sprintf("--memory=%s", spec.IgniteConfig().Memory),
		fmt.Sprintf("--size=%s", spec.IgniteConfig().DiskSize),
		fmt.Sprintf("--kernel-image=%s", spec.IgniteConfig().Kernel),
		fmt.Sprintf("--ssh=%s", pubKeyPath),
	}

	if copyFiles := spec.IgniteConfig().CopyFiles; copyFiles != nil {
		runArgs = append(runArgs, setupCopyFiles(copyFiles)...)
	}

	for _, mapping := range spec.PortMappings {
		if mapping.HostPort == 0 {
			// If not defined, set the host port to a random free ephemeral port
			var err error
			if mapping.HostPort, err = freePort(); err != nil {
				return "", err
			}
		} else {
			// If defined, apply an offset so all VMs won't use the same port
			mapping.HostPort += portOffset
		}

		runArgs = append(runArgs, fmt.Sprintf("--ports=%d:%d", int(mapping.HostPort), mapping.ContainerPort))
	}

	// Increment portOffset per-machine
	portOffset++

	_, err = exec.ExecuteCommand(execName, runArgs...)
	return "", err
}

// setupCopyFiles formats the files to copy over to Ignite flags
func setupCopyFiles(copyFiles map[string]string) []string {
	ret := make([]string, 0, len(copyFiles))
	for k, v := range copyFiles {
		ret = append(ret, fmt.Sprintf("--copy-files=%s:%s", toAbs(k), v))
	}

	return ret
}

func toAbs(p string) string {
	if ap, err := filepath.Abs(p); err == nil {
		return ap
	}

	// If Abs reports an error, just return the given path as-is
	return p
}

// IsCreated checks if the VM with the given name is created
func IsCreated(name string) bool {
	return exec.Command(execName, "inspect", "vm", name).Run() == nil
}

// IsStarted checks if the VM with the given name is running
func IsStarted(name string) bool {
	vm, err := PopulateMachineDetails(name)
	if err != nil {
		return false
	}

	return vm.Status.Running
}

// freePort requests a free/open ephemeral port from the kernel
// Heavily inspired by https://github.com/phayes/freeport/blob/master/freeport.go
func freePort() (uint16, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return uint16(l.Addr().(*net.TCPAddr).Port), nil
}
