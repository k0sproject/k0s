/*
Copyright 2022 k0s authors

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

package iptablesutils_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/iptablesutils"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectHostIPTablesMode(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)

	writeScript := func(t *testing.T, parentDir, fileName, content string) {
		path := filepath.Join(parentDir, fileName)
		content = "#!" + sh + " -eu\n\n" + content
		require.NoError(t, file.WriteContentAtomically(path, []byte(content), 0755))

		if runtime.GOOS == "windows" {
			// Add a shim for cmd.exe. Parameter forwarding is imperfect, but
			// sufficient for these tests.
			require.NoError(t, file.WriteContentAtomically(path+".cmd", []byte(fmt.Sprintf("@%q %q %%*", sh, path)), 0755))
		}
	}

	writeXtables := func(t *testing.T, parentDir, mode, v4Script, v6Script string) {
		content := fmt.Sprintf(
			"case \"$1\" in iptables-save) %s ;; ip6tables-save) %s ;; *) exit 1 ;; esac",
			v4Script, v6Script,
		)

		writeScript(t, parentDir, fmt.Sprintf("xtables-%s-multi", mode), content)
	}

	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)

	t.Run("iptables_not_found", func(t *testing.T) {
		binDir := t.TempDir()

		_, err := iptablesutils.DetectHostIPTablesMode(binDir)

		var execErr *exec.Error
		require.ErrorAs(t, err, &execErr)
		assert.Equal(t, "iptables", execErr.Name)
		assert.ErrorIs(t, execErr.Err, exec.ErrNotFound)
	})

	t.Run("xtables_nft", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft",
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
		)

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeNFT, mode)
	})

	t.Run("xtables_legacy", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "legacy",
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
		)

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeLegacy, mode)
	})

	t.Run("xtables_nft_over_legacy", func(t *testing.T) {
		binDir := t.TempDir()

		writeXtables(t, binDir, "nft",
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 1),
		)
		writeXtables(t, binDir, "legacy",
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 3),
			strings.Repeat("echo KUBE-IPTABLES-HINT\n", 3),
		)

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeNFT, mode)
	})

	t.Run("xtables_legacy_over_nft_more_entries", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft",
			strings.Repeat("echo FOOBAR\n", 1),
			strings.Repeat("echo FOOBAR\n", 1),
		)
		writeXtables(t, binDir, "legacy",
			strings.Repeat("echo FOOBAR\n", 1),
			strings.Repeat("echo FOOBAR\n", 2),
		)

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeLegacy, mode)
	})

	t.Run("fallback_to_iptables_if_xtables_nft_over_legacy_more_entries", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft",
			strings.Repeat("echo FOOBAR\n", 1),
			strings.Repeat("echo FOOBAR\n", 2),
		)
		writeXtables(t, binDir, "legacy",
			strings.Repeat("echo FOOBAR\n", 1),
			strings.Repeat("echo FOOBAR\n", 1),
		)

		_, err := iptablesutils.DetectHostIPTablesMode(binDir)
		var execErr *exec.Error
		require.ErrorAs(t, err, &execErr)
		assert.Equal(t, "iptables", execErr.Name)
		assert.ErrorIs(t, execErr.Err, exec.ErrNotFound)
	})

	t.Run("xtables_nft_fails", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft", "exit 1", "exit 1")
		writeXtables(t, binDir, "legacy", "exit 1", "echo KUBE-IPTABLES-HINT")

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeLegacy, mode)
	})

	t.Run("xtables_legacy_fails", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft", "exit 1", "echo KUBE-IPTABLES-HINT")
		writeXtables(t, binDir, "legacy", "exit 1", "exit 1")

		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeNFT, mode)
	})

	t.Run("xtables_fails", func(t *testing.T) {
		binDir := t.TempDir()
		writeXtables(t, binDir, "nft", "exit 99", "exit 88")
		writeXtables(t, binDir, "legacy", "exit 77", "exit 66")

		_, err := iptablesutils.DetectHostIPTablesMode(binDir)
		var composite interface{ Unwrap() []error }
		require.ErrorAs(t, err, &composite, "No wrapped errors")
		errs := composite.Unwrap()
		require.Len(t, errs, 3)
		assert.ErrorIs(t, errs[0], exec.ErrNotFound)
		assert.ErrorContains(t, errs[1], "99")
		assert.ErrorContains(t, errs[1], "88")
		assert.ErrorContains(t, errs[2], "77")
		assert.ErrorContains(t, errs[2], "66")
	})

	binDir := t.TempDir()
	writeScript(t, pathDir, "iptables", "")
	writeXtables(t, binDir, "nft", "", "")
	writeXtables(t, binDir, "legacy", "", "")

	t.Run("iptables_legacy", func(t *testing.T) {
		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeLegacy, mode)
	})

	writeScript(t, pathDir, "iptables", "echo foo-nf_tables-bar")

	t.Run("iptables_nft", func(t *testing.T) {
		mode, err := iptablesutils.DetectHostIPTablesMode(binDir)
		require.NoError(t, err)
		assert.Equal(t, iptablesutils.ModeNFT, mode)
	})

	writeScript(t, pathDir, "iptables", "exit 1")

	t.Run("iptables_broken", func(t *testing.T) {
		_, err := iptablesutils.DetectHostIPTablesMode(binDir)
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 1, exitErr.ExitCode())
	})
}

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}
