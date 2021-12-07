package cluster

import (
	"bytes"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/footloose/pkg/docker"
	"github.com/weaveworks/footloose/pkg/exec"
)

// run runs a command. It will output the combined stdout/error on failure.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		// log error output if there was any
		for _, line := range output {
			log.Error(line)
		}
	}
	return err
}

// Run a command in a container. It will output the combined stdout/error on failure.
func containerRun(nameOrID string, name string, args ...string) error {
	exe := docker.ContainerCmder(nameOrID)
	cmd := exe.Command(name, args...)
	output, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		// log error output if there was any
		for _, line := range output {
			log.WithField("machine", nameOrID).Error(line)
		}
	}
	return err
}

func containerRunShell(nameOrID string, script string) error {
	return containerRun(nameOrID, "/bin/bash", "-c", script)
}

func copy(nameOrID string, content []byte, path string) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("cat <<__EOF | tee -a %s\n", path))
	buf.Write(content)
	buf.WriteString("__EOF")
	return containerRunShell(nameOrID, buf.String())
}
