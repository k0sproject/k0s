// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Helpers around OCI images used in the integration tests.
package ociimages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	// The nginx HTTP server.
	//
	// renovate: versioning=regex:^(?<major>\d+)\.(?<minor>\d+)\.(?<patch>\d+)-alpine(?<build>\d+)\.(?<revision>\d+)$
	Nginx = "docker.io/library/nginx:1.31.6-alpine3.24@sha256:adad2ae9204d0fd7a34f40299bc838c3782be1293b10005eac4315ff5a1abf4e"
)

// Returns the default Alpine image to be used.
func Alpine(ctx context.Context) (string, error) {
	return makefileVariable(ctx, "embedded-bins", "alpine_image")
}

// Returns the sonobuoy image to run the Kubernetes conformance tests.
func Sonobuoy(ctx context.Context) (string, error) {
	return makefileVariable(ctx, "inttest", "sonobuoy_image")
}

// Queries a variable from the Makefile.variables file in the given directory
// via vars.sh.
func makefileVariable(ctx context.Context, from, name string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to determine the source file location")
	}

	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	cmd := exec.CommandContext(ctx, filepath.Join(repoRoot, "vars.sh"), "FROM="+from, name)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to query Makefile variable %s: %w", name, err)
	}

	value, _, _ := bytes.Cut(out, []byte{'\n'})
	if len(value) < 1 {
		return "", fmt.Errorf("makefile variable %s is empty", name)
	}

	return string(value), nil
}
