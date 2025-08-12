//go:build hack

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	genimages "github.com/k0sproject/k0s/hack/gen-test-images-list"
)

func main() {
	if err := genimages.GenImagesList(os.Args[0], os.Args[1:]...); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
