// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path"
	"strings"

	"github.com/k0sproject/k0s/cmd"
	internallog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/internal/supervised"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

//go:generate make codegen

func init() {
	internallog.InitLogging()
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	supervisor.ShutdownHelperHook()
	defer internallog.ShutdownLogging()

	// Make embedded commands work through symlinks such as /usr/local/bin/kubectl (or k0s-kubectl)
	progN := strings.TrimPrefix(path.Base(os.Args[0]), "k0s-")
	switch progN {
	case "kubectl", "ctr":
		os.Args = append([]string{"k0s", progN}, os.Args[1:]...)
	}

	return supervised.Run(cmd.Execute)
}
