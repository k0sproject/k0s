// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import "github.com/k0sproject/k0s/pkg/leaderelection"

// Interface is the common leader elector component to manage each controller leader status.
type Interface interface {
	// Deprecated: Use [Interface.CurrentStatus] instead.
	IsLeader() bool

	// Deprecated: Use [Interface.CurrentStatus] instead.
	AddAcquiredLeaseCallback(fn func())

	// Deprecated: Use [Interface.CurrentStatus] instead.
	AddLostLeaseCallback(fn func())

	// CurrentStatus is this leader elector's [leaderelection.StatusFunc].
	CurrentStatus() (status leaderelection.Status, expired <-chan struct{})
}
