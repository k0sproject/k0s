package cleanup

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/sirupsen/logrus"
)

type cni struct {
	Config   *Config
	toRemove []string
}

// Name returns the name of the step
func (c *cni) Name() string {
	return "CNI leftovers cleanup step"
}

// NeedsToRun checks if there are and CNI leftovers
func (c *cni) NeedsToRun() bool {
	files := []string{
		"/etc/cni/net.d/10-calico.conflist",
		"/etc/cni/net.d/calico-kubeconfig",
		"/etc/cni/net.d/10-kuberouter.conflist",
	}

	for _, file := range files {
		if util.FileExists(file) {
			c.toRemove = append(c.toRemove, file)
		}
	}
	return len(c.toRemove) > 0
}

// Run removes found CNI leftovers
func (c *cni) Run() error {
	return removeCNILeftovers(c.toRemove)
}

func removeCNILeftovers(files []string) error {
	var msg []error

	for _, file := range files {
		if util.FileExists(file) {
			if err := os.Remove(file); err != nil {
				logrus.Debug("failed to remove", file, err)
				msg = append(msg, err)
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("error occured while removing CNI leftovers: %v", msg)
	}
	return nil
}
