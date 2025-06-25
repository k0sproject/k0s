//go:build hack

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	genbindata "github.com/k0sproject/k0s/hack/gen-bindata"
)

func main() {
	if err := genbindata.GenBindata(os.Args[0], os.Args[1:]...); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
