//go:build !linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

// Keepalived doesn't work on windows, so we cannot implement it at all.
// Just create the interface so that the CI doesn't complain.
type Keepalived struct {
	K0sVars         *config.CfgVars
	Config          *k0sAPI.KeepalivedSpec
	DetailedLogging bool
	LogConfig       bool
	APIPort         int
	KubeConfigPath  string
}

func (k *Keepalived) Init(context.Context) error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}

func (k *Keepalived) Start(context.Context) error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}

func (k *Keepalived) Stop() error {
	return fmt.Errorf("%w: CPLB is not supported on %s", errors.ErrUnsupported, runtime.GOOS)
}
