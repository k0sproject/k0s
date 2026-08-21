// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// list-images prints every container image (and its version) that k0s may
// pull, across all platforms and configurations. Unlike `k0s airgap
// list-images`, it doesn't need a running node or a linux host to build/run,
// since it only touches pkg/airgap and pkg/apis/k0s/v1beta1 directly.
package main

import (
	"fmt"
	"slices"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func main() {
	spec := v1beta1.DefaultClusterSpec()

	// Mirrors the platforms built by the airgap-image-bundle-*.tar Makefile
	// targets.
	seen := make(map[string]struct{})
	var uris []string
	for _, platform := range []imagespecv1.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm"},
		{OS: "linux", Architecture: "riscv64"},
		{OS: "windows", Architecture: "amd64"},
	} {
		for _, uri := range airgap.GetImageURIs(airgap.TargetEnv{Platform: platform, Spec: spec}, true) {
			if _, ok := seen[uri]; !ok {
				seen[uri] = struct{}{}
				uris = append(uris, uri)
			}
		}
	}

	slices.Sort(uris)
	for _, uri := range uris {
		fmt.Println(uri)
	}
}
