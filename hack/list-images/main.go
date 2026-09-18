// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// list-images prints every container image (and its version) that k0s may
// pull, across all configurations. Unlike `k0s airgap list-images`, it
// doesn't need a running node or a linux host to build/run, since it only
// touches pkg/airgap and pkg/apis/k0s/v1beta1 directly.
package main

import (
	"fmt"
	"slices"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

func main() {
	spec := v1beta1.DefaultClusterSpec()

	uris := airgap.GetImageURIs(spec, true)

	slices.Sort(uris)
	for _, uri := range uris {
		fmt.Println(uri)
	}
}
