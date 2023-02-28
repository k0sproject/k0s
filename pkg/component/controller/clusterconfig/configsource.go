/*
Copyright 2020 k0s authors

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

package clusterconfig

import (
	"context"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
)

type ConfigSource interface {
	// Release allows the config source to start sending config updates
	Release(context.Context)
	// ResultChan provides the result channel where config updates are pushed by the source on it is released
	ResultChan() <-chan *v1beta1.ClusterConfig
	// Stop stops sending config events
	Stop()
	// NeedToStoreInitialConfig tells the configsource user if the initial config should be stored in the api or not
	NeedToStoreInitialConfig() bool
}
