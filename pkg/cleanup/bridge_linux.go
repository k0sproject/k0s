package cleanup

import (
	"fmt"
	"runtime"

	"github.com/vishvananda/netlink"
)

type bridge struct{}

// Name returns the name of the step
func (b *bridge) Name() string {
	return "kube-bridge leftovers cleanup step"
}

// NeedsToRun checks if there are and kube-bridge leftovers
func (b *bridge) NeedsToRun() bool {
	return true
}

// Run removes found kube-bridge leftovers
func (b *bridge) Run() error {
	if runtime.GOOS == "windows" {
		return nil
	}

	lnks, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to get link list from netlink: %v", err)
	}

	for _, l := range lnks {
		if l.Attrs().Name == "kube-bridge" {
			err := netlink.LinkDel(l)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
