package cleanup

import (
	"fmt"
	"runtime"

	"github.com/vishvananda/netlink"
)

type bridge struct {
	link netlink.Link
}

// Name returns the name of the step
func (b *bridge) Name() string {
	return "kube-bridge leftovers cleanup step"
}

// NeedsToRun checks if there are and kube-bridge leftovers
func (b *bridge) NeedsToRun() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	linkName := "kube-bridge"
	lnks, err := netlink.LinkList()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}

	for _, l := range lnks {
		if l.Attrs().Name == linkName {
			b.link = l
			return true
		}

	}

	return false
}

// Run removes found kube-bridge leftovers
func (b *bridge) Run() error {
	err := netlink.LinkDel(b.link)
	if err != nil {
		return err
	}
	return nil
}
