// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"net/url"
)

type ContainerRuntime interface {
	Ping(ctx context.Context) error
	ListContainers(ctx context.Context) ([]string, error)
	RemoveContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
}

func NewContainerRuntime(runtimeEndpoint *url.URL) ContainerRuntime {
	return &CRIRuntime{runtimeEndpoint.String()}
}
