//go:build !windows
// +build !windows

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

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
)

type NodesetupHelper struct {
}

var _ manager.Component = (*NodesetupHelper)(nil)

func (c NodesetupHelper) Init(_ context.Context) error {
	panic("stub component is used: NodesetupHelper which is implemented only for windows")
}

func (c NodesetupHelper) Start(_ context.Context) error {
	panic("stub component is used: NodesetupHelper which is implemented only for windows")
}

func (c NodesetupHelper) Stop() error {
	panic("stub component is used: NodesetupHelper which is implemented only for windows")
}
