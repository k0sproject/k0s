package config

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Volume is a volume that can be attached to a Machine.
type Volume struct {
	// Type is the volume type. One of "bind" or "volume".
	Type string `json:"type"`
	// Source is the volume source.
	// With type=bind, the volume source is a directory or a file in the host
	// filesystem.
	// With type=volume, source is either the name of a docker volume or "" for
	// anonymous volumes.
	Source string `json:"source"`
	// Destination is the mount point inside the container.
	Destination string `json:"destination"`
	// ReadOnly specifies if the volume should be read-only or not.
	ReadOnly bool `json:"readOnly"`
}

// PortMapping describes mapping a port from the machine onto the host.
type PortMapping struct {
	// Protocol is the layer 4 protocol for this mapping. One of "tcp" or "udp".
	// Defaults to "tcp".
	Protocol string `json:"protocol,omitempty"`
	// Address is the host address to bind to. Defaults to "0.0.0.0".
	Address string `json:"address,omitempty"`
	// HostPort is the base host port to map the containers ports to. As we
	// configure a number of machine replicas, each machine will use HostPort+i
	// where i is between 0 and N-1, N being the number of machine replicas. If 0,
	// a local port will be automatically allocated.
	HostPort uint16 `json:"hostPort,omitempty"`
	// ContainerPort is the container port to map.
	ContainerPort uint16 `json:"containerPort"`
}

// Machine is the machine configuration.
type Machine struct {
	// Name is the machine name.
	//
	// When used in a MachineReplicas object, eg. in footloose.yaml config files,
	// this field a format string. This format string needs to have a '%d', which
	// is populated by the machine index, a number between 0 and N-1, N being the
	// Count field of MachineReplicas. Name will default to "node%d"
	//
	// This name will also be used as the machine hostname.
	Name string `json:"name"`
	// Image is the container image to use for this machine.
	Image string `json:"image"`
	// Privileged controls whether to start the Machine as a privileged container
	// or not. Defaults to false.
	Privileged bool `json:"privileged,omitempty"`
	// Volumes is the list of volumes attached to this machine.
	Volumes []Volume `json:"volumes,omitempty"`
	// Networks is the list of user-defined docker networks this machine is
	// attached to. These networks have to be created manually before creating the
	// containers via "docker network create mynetwork"
	Networks []string `json:"networks,omitempty"`
	// PortMappings is the list of ports to expose to the host.
	PortMappings []PortMapping `json:"portMappings,omitempty"`
	// Cmd is a cmd which will be run in the container.
	Cmd string `json:"cmd,omitempty"`
	// PublicKey is the name of the public key to upload onto the machine for root
	// SSH access.
	PublicKey string `json:"publicKey,omitempty"`

	// Backend specifies the runtime backend for this machine
	Backend string `json:"backend,omitempty"`
	// Ignite specifies ignite-specific options
	Ignite *Ignite `json:"ignite,omitempty"`
}

func (m *Machine) IgniteConfig() Ignite {
	i := Ignite{}
	if m.Ignite != nil {
		i = *m.Ignite
	}
	if i.CPUs == 0 {
		i.CPUs = 2
	}
	if len(i.Memory) == 0 {
		i.Memory = "1GB"
	}
	if len(i.DiskSize) == 0 {
		i.DiskSize = "4GB"
	}
	if len(i.Kernel) == 0 {
		i.Kernel = "weaveworks/ignite-kernel:4.19.47"
	}
	return i
}

// Ignite holds the ignite-specific configuration
type Ignite struct {
	// CPUs specify the number of vCPUs to use. Default: 2
	CPUs uint64 `json:"cpus,omitempty"`
	// Memory specifies the amount of RAM the VM should have. Default: 1GB
	Memory string `json:"memory,omitempty"`
	// DiskSize specifies the amount of disk space the VM should have. Default: 4GB
	DiskSize string `json:"diskSize,omitempty"`
	// Kernel specifies an OCI image to use for the kernel overlay
	Kernel string `json:"kernel,omitempty"`
	// Files to copy to the VM
	CopyFiles map[string]string `json:"copyFiles,omitempty"`
}

// validate checks basic rules for Machine's fields
func (conf Machine) validate() error {
	validName := strings.Contains(conf.Name, "%d")
	if !validName {
		log.Warnf("Machine conf validation: machine name %v is not valid, it should contains %%d", conf.Name)
		return fmt.Errorf("Machine configuration not valid")
	}
	return nil
}
