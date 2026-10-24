//go:build !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

type BGP struct {
	Config         *k0sAPI.BGPSpec
	KubeConfigPath string
	Address        string
}

func (b *BGP) Init(context.Context) error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}

func (b *BGP) Start(context.Context) error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}

func (b *BGP) Stop() error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}
