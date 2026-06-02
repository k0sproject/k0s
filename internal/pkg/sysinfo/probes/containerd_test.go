// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingReporter collects outcomes from probes for assertions in tests.
type recordingReporter struct {
	passes     []string
	warnings   []string
	rejections []string
	errors     []error
}

func (r *recordingReporter) Pass(d probes.ProbeDesc, prop probes.ProbedProp) error {
	r.passes = append(r.passes, d.DisplayName())
	return nil
}

func (r *recordingReporter) Warn(d probes.ProbeDesc, _ probes.ProbedProp, msg string) error {
	r.warnings = append(r.warnings, msg)
	return nil
}

func (r *recordingReporter) Reject(d probes.ProbeDesc, _ probes.ProbedProp, msg string) error {
	r.rejections = append(r.rejections, msg)
	return nil
}

func (r *recordingReporter) Error(d probes.ProbeDesc, err error) error {
	r.errors = append(r.errors, err)
	return nil
}

func TestRequireContainerdV2ConfigSnippets(t *testing.T) {
	t.Run("missing directory passes silently", func(t *testing.T) {
		dir := t.TempDir()
		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, filepath.Join(dir, "nonexistent"))

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Empty(t, rep.rejections)
		assert.Empty(t, rep.errors)
	})

	t.Run("empty directory passes silently", func(t *testing.T) {
		dir := t.TempDir()

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Empty(t, rep.rejections)
		assert.Empty(t, rep.errors)
	})

	t.Run("valid version 3 file passes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "valid.toml", `
version = 3

[plugins."io.containerd.cri.v1.runtime"]
  sandbox_image = "registry.k8s.io/pause:3.9"
`)

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Empty(t, rep.rejections)
		assert.Empty(t, rep.errors)
		assert.Len(t, rep.passes, 2) // one for the directory, one for the file
	})

	t.Run("file without version field passes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "noversion.toml", `
[plugins."io.containerd.cri.v1.runtime"]
  sandbox_image = "registry.k8s.io/pause:3.9"
`)

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Empty(t, rep.rejections)
		assert.Empty(t, rep.errors)
	})

	t.Run("file with version 2 is rejected with remediation", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "nvidia.toml", "version = 2\n")

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		require.Len(t, rep.rejections, 1)

		msg := rep.rejections[0]
		assert.Contains(t, msg, "expected 3, got 2")
	})

	t.Run("file with v1 CRI plugin key is rejected with remediation", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "cni.toml", `
version = 3

[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "registry.k8s.io/pause:3.9"
`)

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		require.Len(t, rep.rejections, 1)

		msg := rep.rejections[0]
		assert.Contains(t, msg, "io.containerd.grpc.v1.cri")
	})

	t.Run("multiple files: only bad ones are rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "good.toml", "version = 3\n")
		writeFile(t, dir, "bad.toml", "version = 2\n")

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Len(t, rep.rejections, 1)
		assert.Contains(t, rep.rejections[0], "expected 3, got 2")
		assert.Len(t, rep.passes, 2) // one for the directory, one for the good file
	})

	t.Run("invalid TOML results in rejection not error", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "corrupt.toml", "this is not [ valid toml !!!")

		p := probes.NewRootProbes()
		probes.RequireContainerdV2ConfigSnippets(p, dir)

		rep := &recordingReporter{}
		require.NoError(t, p.Probe(rep))
		assert.Len(t, rep.rejections, 1)
		assert.Contains(t, rep.rejections[0], "failed to parse TOML")
		assert.Empty(t, rep.errors)
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}
