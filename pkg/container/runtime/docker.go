package runtime

import (
	"github.com/pkg/errors"
	"os/exec"
	"strings"
)

var _ ContainerRuntime = &DockerRuntime{}

type DockerRuntime struct {
	criSocketPath string
}

func (d *DockerRuntime) ListContainers() ([]string, error) {
	out, err := exec.Command("docker", "--host", d.criSocketPath, "ps", "-a", "--filter", "name=k8s_", "-q").CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers: output: %s, error", string(out))
	}
	return strings.Fields(string(out)), nil
}

func (d *DockerRuntime) RemoveContainer(id string) error {
	out, err := exec.Command("docker", "--host", d.criSocketPath, "rm", "--volumes", id).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to remove container %s: output: %s, error", id, string(out))
	}
	return nil
}

func (d *DockerRuntime) StopContainer(id string) error {
	out, err := exec.Command("docker", "--host", d.criSocketPath, "stop", id).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to stop running container %s: output: %s, error", id, string(out))
	}
	return nil
}
