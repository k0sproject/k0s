// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"slices"
	"strings"
)

// manageDiscoveryAuthentication reports whether k0s may configure anonymous authentication.
// Users who configure authentication themselves, also via raw arguments, need to allow the discovery path on their own.
func manageDiscoveryAuthentication(args map[string]string, rawArgs []string) bool {
	var names []string
	for name, value := range args {
		// Empty values don't count, except for the OIDC flags.
		if value != "" || strings.HasPrefix(name, "oidc-") {
			names = append(names, name)
		}
	}
	for _, arg := range rawArgs {
		// Values of flags in separate arguments don't start with a dash. Raw arguments count even without a value.
		if strings.HasPrefix(arg, "-") {
			name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			names = append(names, name)
		}
	}
	// User settings win. kube-apiserver also refuses to combine the OIDC flags with an authentication configuration.
	return !slices.ContainsFunc(names, func(name string) bool {
		return name == "authentication-config" || name == "anonymous-auth" || strings.HasPrefix(name, "oidc-")
	})
}
