// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
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

func getSourceVip() (string, error) {
	// make it use winExecute and powershell
	var vip string

	err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		ep, err := hcsshim.GetHNSEndpointByName("Calico_ep")
		if err != nil {
			logrus.WithError(err).Warn("can't get Calico_ep endpoint")
			return false, nil
		}
		vip = ep.IPAddress.String()
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return vip, nil
}

func winExecute(args ...string) error {
	ps := NewPowershell()

	r := ps.execute(args...)
	return r
}
