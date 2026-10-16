// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

// The authentication configuration that lets anonymous users read the cluster-info ConfigMap only.
// Bootstrap token discovery reads it this way. RBAC still decides whether anonymous users may get it.
const discoveryAuthenticationConfig = `apiVersion: apiserver.config.k8s.io/v1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /api/v1/namespaces/kube-public/configmaps/cluster-info
`

// manageDiscoveryAuthentication reports whether k0s may configure anonymous authentication.
// Users who configure authentication themselves, also via raw arguments, need to allow the discovery path on their own.
func manageDiscoveryAuthentication(args map[string]string, rawArgs []string) bool {
	if args["authentication-config"] != "" || args["anonymous-auth"] != "" {
		return false
	}
	names := slices.Collect(maps.Keys(args))
	for _, arg := range rawArgs {
		// Values of flags in separate arguments don't start with a dash.
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		// Raw arguments count even without a value.
		if name == "authentication-config" || name == "anonymous-auth" {
			return false
		}
		names = append(names, name)
	}
	// kube-apiserver refuses to combine the OIDC flags with an authentication configuration.
	return !slices.ContainsFunc(names, func(name string) bool { return strings.HasPrefix(name, "oidc-") })
}

// discoveryAuthenticationConfigPath returns where the authentication configuration goes.
func discoveryAuthenticationConfigPath(k0sVars *config.CfgVars) string {
	return filepath.Join(k0sVars.DataDir, "authentication-config.yaml")
}

// writeDiscoveryAuthenticationConfig writes the authentication configuration to the path.
func writeDiscoveryAuthenticationConfig(path string) error {
	// kube-apiserver doesn't run as root.
	return file.AtomicWithTarget(path).WithPermissions(constant.CertMode).WriteString(discoveryAuthenticationConfig)
}
