package cleanup

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type cni struct{}

// Name returns the name of the step
func (c *cni) Name() string {
	return "CNI leftovers cleanup step"
}

// Run removes found CNI leftovers
func (c *cni) Run() error {
	var msg []error

	files := []string{
		"/etc/cni/net.d/10-calico.conflist",
		"/etc/cni/net.d/calico-kubeconfig",
		"/etc/cni/net.d/10-kuberouter.conflist",
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			if !os.IsNotExist(err) {
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
