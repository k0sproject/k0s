/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
