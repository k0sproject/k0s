/*
Copyright 2023 k0s authors

Licensed under the Apache License, Version 2.0 (the &quot;License&quot;);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an &quot;AS IS&quot; BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
