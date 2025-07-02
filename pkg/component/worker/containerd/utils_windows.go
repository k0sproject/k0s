// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"os"
	"os/exec"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

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

	err := retry.Do(func() error {
		ep, err := hcsshim.GetHNSEndpointByName("Calico_ep")
		if err != nil {
			logrus.WithError(err).Warn("can't get Calico_ep endpoint")
			return err
		}
		vip = ep.IPAddress.String()
		return nil
	}, retry.Delay(time.Second*5))
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
