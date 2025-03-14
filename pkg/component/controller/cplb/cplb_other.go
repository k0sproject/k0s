//go:build !linux

/*
Copyright 2024 k0s authors

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
