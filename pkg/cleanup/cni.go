package cleanup

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/file"
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

	for _, f := range files {
		if file.Exists(f) {
			c.toRemove = append(c.toRemove, f)
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

	for _, f := range files {
		if file.Exists(f) {
			if err := os.Remove(f); err != nil {
				logrus.Debug("failed to remove", f, err)
				msg = append(msg, err)
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("error occured while removing CNI leftovers: %v", msg)
	}
	return nil
}
