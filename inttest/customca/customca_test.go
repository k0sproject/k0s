// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package customca

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type CustomCASuite struct {
	common.BootlooseSuite
}

func (s *CustomCASuite) TestK0sGetsUp() {
	// Create an custom certificate to prove that k0s manage to work with it
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	err = ssh.Exec(s.Context(), "sh -e", common.SSHStreams{
		//nolint:dupword // this is a script
		In: strings.NewReader(fmt.Sprintf(`
			K0S_PATH=%q
			IP_ADDRESS=%q
			mkdir -p /var/lib/k0s/pki /var/lib/k0s/manifests/test
			apk add openssl
			openssl genrsa -out /var/lib/k0s/pki/ca.key 4096
			openssl req -x509 -new -nodes -key /var/lib/k0s/pki/ca.key -sha256 -days 365 -out /var/lib/k0s/pki/ca.crt -subj "/CN=Test CA"
			"$K0S_PATH" token pre-shared --cert /var/lib/k0s/pki/ca.crt --url https://"$IP_ADDRESS":6443/ --out /var/lib/k0s/manifests/test
		`, s.K0sFullPath, s.GetControllerIPAddress(0))),
	})
	s.Require().NoError(err)

	cert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/ca.crt")
	s.Require().NoError(err)
	token, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/manifests/test/token_* && rm /var/lib/k0s/manifests/test/token_*")
	s.Require().NoError(err)

	s.NoError(s.InitController(0))

	s.NoError(s.RunWorkersWithToken(token))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err, "Failed to obtain Kubernetes client")

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	newCert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/ca.crt")
	s.Require().NoError(err)
	s.Equal(cert, newCert)
}

func TestCustomCASuite(t *testing.T) {
	s := CustomCASuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
