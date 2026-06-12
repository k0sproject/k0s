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

func (s *CustomCASuite) initCA(ssh *common.SSHConnection, dir, cn string) string {
	err := ssh.Exec(s.Context(), "sh -e", common.SSHStreams{
		//nolint:dupword // this is a script
		In: strings.NewReader(strings.NewReplacer("{dir}", dir, "{cn}", cn).Replace(`
			mkdir -p {dir}
			command -v openssl || apk add openssl
			openssl genrsa -out {dir}/ca.key 4096
			openssl req -x509 -new -nodes -key {dir}/ca.key -sha256 -days 365 -out {dir}/ca.crt -subj "/CN={cn}"
		`)),
	})
	s.Require().NoError(err)

	cert, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("cat %s/ca.crt", dir))
	s.Require().NoError(err)

	return cert
}

func (s *CustomCASuite) TestK0sGetsUp() {
	// Create an custom certificate to prove that k0s manage to work with it
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	caCert := s.initCA(ssh, "/var/lib/k0s/pki", "Test CA")
	etcdCACert := s.initCA(ssh, "/var/lib/k0s/pki/etcd", "Test Etcd CA")

	err = ssh.Exec(s.Context(), "sh -e", common.SSHStreams{
		In: strings.NewReader(fmt.Sprintf(`
			K0S_PATH=%q
			IP_ADDRESS=%q
			mkdir -p /var/lib/k0s/manifests/test
			"$K0S_PATH" token pre-shared --cert /var/lib/k0s/pki/ca.crt --url https://"$IP_ADDRESS":6443/ --out /var/lib/k0s/manifests/test
		`, s.K0sFullPath, s.GetControllerIPAddress(0))),
	})
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
	s.Equal(caCert, newCert)

	newEtcdCert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/etcd/ca.crt")
	s.Require().NoError(err)
	s.Equal(etcdCACert, newEtcdCert)

	// Validate that k0s renews certificates with custom CA
	k0sAPICert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/k0s-api.crt")
	s.Require().NoError(err, "Failed to obtain k0s-api certificate")

	etcdServerCert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/etcd/server.crt")
	s.Require().NoError(err, "Failed to obtain etcd server certificate")

	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	s.Require().NoError(s.StartController(s.ControllerNode(0)))

	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0))) // Wait for the k0s join API to be ready after restart
	newK0sAPICert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/k0s-api.crt")
	s.Require().NoError(err, "Failed to obtain new k0s certificate")
	s.Require().NotEqual(k0sAPICert, newK0sAPICert, "k0s-api certificate was not renewed")

	newEtcdServerCert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/etcd/server.crt")
	s.Require().NoError(err, "Failed to obtain new etcd server certificate")
	s.Require().NotEqual(etcdServerCert, newEtcdServerCert, "etcd server certificate was not renewed")
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
