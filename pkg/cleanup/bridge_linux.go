// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func newBridgeStep() Step {
	return linuxBridge{}
}

type linuxBridge struct{}

// Name returns the name of the step
func (linuxBridge) Name() string {
	return "kube-bridge leftovers cleanup step"
}

// Run removes found kube-bridge leftovers
func (linuxBridge) Run() error {
	lnks, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to get link list from netlink: %w", err)
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
