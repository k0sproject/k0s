//go:build !linux && !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"errors"
	"fmt"
	"runtime"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

func buildSteps(bool, *config.CfgVars, *k0sv1beta1.SystemUser, string) ([]Step, error) {
	return nil, fmt.Errorf("%w on %s", errors.ErrUnsupported, runtime.GOOS)
}
