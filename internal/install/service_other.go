//go:build !linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import "github.com/kardianos/service"

func configureServicePlatform(s service.Service, svcConfig *service.Config) {
	// no-op
}
