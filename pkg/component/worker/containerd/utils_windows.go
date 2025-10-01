// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
)

func Address(_ string) string {
	return `\\.\pipe\containerd-containerd`
}

func Endpoint(runDir string) *url.URL {
	return &url.URL{
		Scheme: "npipe",
		Path:   filepath.ToSlash(Address(runDir)),
	}
}

// PowerShell struct
type PowerShell struct {
	powerShell string
	err        error
}

// New create new session
func NewPowershell() *PowerShell {
	ps, err := exec.LookPath("powershell.exe")
	return &PowerShell{
		powerShell: ps,
		err:        err,
	}
}

func (p *PowerShell) execute(args ...string) error {
	if p.err != nil {
		return p.err
	}
	args = append([]string{"-NoProfile", "-NonInteractive"}, args...)
	cmd := exec.Command(p.powerShell, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func winExecute(args ...string) error {
	ps := NewPowershell()

	r := ps.execute(args...)
	return r
}
