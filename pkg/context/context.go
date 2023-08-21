/*
Copyright 2023 k0s authors

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

package context

import (
	"context"

	k0sapi "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

type Key string

const (
	ContextNodeConfigKey    Key = "k0s_node_config"
	ContextClusterConfigKey Key = "k0s_cluster_config"
)

func FromContext[out any](ctx context.Context, key Key) *out {
	v, ok := ctx.Value(key).(*out)
	if !ok {
		return nil
	}
	return v
}

func GetNodeConfig(ctx context.Context) *k0sapi.ClusterConfig {
	cfg, ok := ctx.Value(ContextNodeConfigKey).(*k0sapi.ClusterConfig)
	if !ok {
		return nil
	}

	return cfg
}
