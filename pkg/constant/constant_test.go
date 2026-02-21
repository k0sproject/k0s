// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"crypto/tls"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/tools/go/packages"
)

func TestConstants(t *testing.T) {
	for _, test := range []struct{ name, constant, varName string }{
		{"KonnectivityImageVersion", KonnectivityImageVersion, "konnectivity"},
		{"KubeProxyImageVersion", KubeProxyImageVersion, "kubernetes"},
		{"KubeProxyWindowsImageVersion", KubeProxyWindowsImageVersion, "kubernetes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected := fmt.Sprintf("^v%s($|-)", regexp.QuoteMeta(getVersion(t, test.varName)))
			assert.Regexp(t, expected, test.constant)
		})
	}

	t.Run("KubernetesMajorMinorVersion", func(t *testing.T) {
		ver := strings.Split(getVersion(t, "kubernetes"), ".")
		require.GreaterOrEqualf(t, len(ver), 2, "failed to spilt Kubernetes version %q", ver)
		kubeMajorMinor := ver[0] + "." + ver[1]
		//nolint:testifylint // kubeMajorMinor _is_ the expected value
		assert.Equal(t, kubeMajorMinor, KubernetesMajorMinorVersion)
	})
}

func TestTLSCipherSuites(t *testing.T) {
	// Verify that the ciphers in use are still considered secure by Go.
	cipherSuites := tls.CipherSuites()
	for _, cipherSuite := range AllowedTLS12CipherSuiteIDs {
		idx := slices.IndexFunc(cipherSuites, func(x *tls.CipherSuite) bool {
			return x.ID == cipherSuite
		})
		if idx < 0 {
			assert.Failf(t, "Not in tls.CipherSuites(), potentially insecure", "(0x%04x) %s", cipherSuite, tls.CipherSuiteName(cipherSuite))
		}
	}
}

func TestKubernetesModuleVersions(t *testing.T) {
	kubernetesVersion := getVersion(t, "kubernetes")

	assertPackageModules(t,
		func(modulePath string) bool {
			switch modulePath {
			// Don't report any version mismatches on the following modules.
			// They have a release cycle which is decoupled from k8s itself.
			case "k8s.io/klog/v2", "k8s.io/kube-openapi", "k8s.io/utils":
				return false

			default:
				return strings.HasPrefix(modulePath, "k8s.io/")
			}
		},
		func(t *testing.T, pkgPath string, module *packages.Module) bool {
			modVer := module.Version
			if module.Path != "k8s.io/kubernetes" {
				// All modules besides Kubernetes itself use v0 instead of v1.
				modVer = strings.Replace(modVer, "v0.", "v1.", 1)
			}

			return !assert.Equal(t, "v"+kubernetesVersion, modVer,
				"Module version for package %s doesn't match: %+#v",
				pkgPath, module,
			)
		},
	)
}

func TestEtcdModuleVersions(t *testing.T) {
	etcdVersion := getVersion(t, "etcd")
	etcdVersionParts := strings.Split(etcdVersion, ".")
	require.GreaterOrEqualf(t, len(etcdVersionParts), 1, "failed to spilt etcd version %q", etcdVersion)

	assertPackageModules(t,
		func(modulePath string) bool {
			return strings.HasPrefix(modulePath, "go.etcd.io/etcd/") &&
				strings.HasSuffix(modulePath, "/v"+etcdVersionParts[0])
		},
		func(t *testing.T, pkgPath string, module *packages.Module) bool {
			return !assert.Equal(t, "v"+etcdVersion, module.Version,
				"Module version for package %s doesn't match: %+#v",
			)
		},
	)
}

func TestContainerdModuleVersions(t *testing.T) {
	containerdVersion := getVersion(t, "containerd")

	assertPackageModules(t,
		func(modulePath string) bool {
			return modulePath == "github.com/containerd/containerd/v2"
		},
		func(t *testing.T, pkgPath string, module *packages.Module) bool {
			return !assert.Equal(t, "v"+containerdVersion, module.Version,
				"Module version for package %s doesn't match: %+#v",
				pkgPath, module,
			)
		},
	)
}

func TestKonnectivityModuleVersions(t *testing.T) {
	konnectivityVersion := getVersion(t, "konnectivity")

	assertPackageModules(t,
		func(modulePath string) bool {
			return strings.HasPrefix(modulePath, "sigs.k8s.io/apiserver-network-proxy/")
		},
		func(t *testing.T, pkgPath string, module *packages.Module) bool {
			return !assert.Equal(t, "v"+konnectivityVersion, module.Version,
				"Module version for package %s doesn't match: %+#v",
				pkgPath, module,
			)
		},
	)
}

func getVersion(t *testing.T, component string) string {
	cmd := exec.Command("sh", "./vars.sh", component+"_version")
	cmd.Dir = filepath.Join("..", "..")

	out, err := cmd.Output()
	require.NoError(t, err)
	require.NotEmptyf(t, out, "failed to get %s version", component)

	trailingNewlines := regexp.MustCompilePOSIX("(\r?\n)+$")
	return string(trailingNewlines.ReplaceAll(out, []byte{}))
}

func assertPackageModules(t *testing.T, filter func(modulePath string) bool, check func(t *testing.T, pkgPath string, module *packages.Module) bool) {
	numMatched := checkPackageModules(t, filter, check)
	assert.NotZero(t, numMatched, "Not a single package passed the filter.")
}

func checkPackageModules(t *testing.T, filter func(modulePath string) bool, check func(t *testing.T, pkgPath string, module *packages.Module) bool) (numMatched uint) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedModule | packages.NeedImports | packages.NeedDeps,
		Logf: t.Logf,
	}, "github.com/k0sproject/k0s")
	require.NoError(t, err)

	failedModules := make(map[string]bool)

	packages.Visit(pkgs, func(p *packages.Package) bool {
		if p.Module != nil && filter(p.Module.Path) {
			actual := p.Module
			for actual.Replace != nil {
				actual = actual.Replace
			}

			if !failedModules[actual.Path] {
				numMatched++
				if !check(t, p.PkgPath, actual) {
					failedModules[actual.Path] = true
				}
			}
		}

		return true
	}, nil)

	return
}
