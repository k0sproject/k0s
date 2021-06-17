package runtime

type ContainerRuntime interface {
	ListContainers() ([]string, error)
	RemoveContainer(id string) error
	StopContainer(id string) error
}

func NewContainerRuntime(runtimeType string, criSocketPath string) ContainerRuntime {
	if runtimeType == "docker" {
		return &DockerRuntime{criSocketPath}
	}
	return &CRIRuntime{criSocketPath}
}
