// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
)

type ConfigSource interface {
	// ResultChan provides the result channel where config updates are pushed by the source on it is released
	ResultChan() <-chan *v1beta1.ClusterConfig

	manager.Component
}
