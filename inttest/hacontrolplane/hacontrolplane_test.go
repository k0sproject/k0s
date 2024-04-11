/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hacontrolplane

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type HAControlplaneSuite struct {
	common.BootlooseSuite
}

func (s *HAControlplaneSuite) getMembers(fromControllerIdx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(s.Context(), s.ControllerNode(fromControllerIdx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	output, err := sshCon.ExecWithOutput(s.Context(), "/usr/local/bin/k0s etcd member-list 2>/dev/null")
	s.T().Logf("k0s etcd member-list output: %s", output)
	s.Require().NoError(err)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.NoError(json.Unmarshal([]byte(output), &members))
	return members.Members
}

func (s *HAControlplaneSuite) makeNodeLeave(executeOnControllerIdx int, peerAddress string) {
	sshCon, err := s.SSH(s.Context(), s.ControllerNode(executeOnControllerIdx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	for i := 0; i < 20; i++ {
		_, err := sshCon.ExecWithOutput(s.Context(), fmt.Sprintf("/usr/local/bin/k0s etcd leave --peer-address %s", peerAddress))
		if err == nil {
			break
		}
		s.T().Logf("retrying k0s etcd leave...")
		time.Sleep(500 * time.Millisecond)
	}
	s.Require().NoError(err)
}

func (s *HAControlplaneSuite) TestDeregistration() {
	// Verify that k0s return failure (https://github.com/k0sproject/k0s/issues/790)
	sshC0, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	_, err = sshC0.ExecWithOutput(s.Context(), "/usr/local/bin/k0s etcd member-list")
	s.Require().Error(err)

	s.NoError(s.InitController(0))
	s.NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.PutFile(s.ControllerNode(1), "/etc/k0s.token", token)
	s.NoError(s.InitController(1, "--token-file=/etc/k0s.token"))
	s.NoError(s.WaitJoinAPI(s.ControllerNode(1)))

	ca0 := s.GetFileFromController(0, "/var/lib/k0s/pki/ca.crt")
	s.Contains(ca0, "-----BEGIN CERTIFICATE-----")

	ca1 := s.GetFileFromController(1, "/var/lib/k0s/pki/ca.crt")
	s.Contains(ca1, "-----BEGIN CERTIFICATE-----")

	s.Equal(ca0, ca1)

	sa0Key := s.GetFileFromController(0, "/var/lib/k0s/pki/sa.key")
	sa1Key := s.GetFileFromController(1, "/var/lib/k0s/pki/sa.key")
	s.Equal(sa0Key, sa1Key)

	sa0Pub := s.GetFileFromController(0, "/var/lib/k0s/pki/sa.pub")
	sa1Pub := s.GetFileFromController(1, "/var/lib/k0s/pki/sa.pub")
	s.Equal(sa0Pub, sa1Pub)

	membersFromMain := s.getMembers(0)
	membersFromJoined := s.getMembers(1)
	s.Equal(membersFromMain, membersFromJoined,
		"etcd cluster members list must be the same across all nodes")
	s.Len(membersFromJoined, 2, "etcd cluster must have exactly 2 members")

	// Restart the second controller with a token to see it comes up
	// It should just ignore the token as there's CA etc already in place
	sshC1, err := s.SSH(s.Context(), s.ControllerNode(1))
	s.Require().NoError(err)
	defer sshC1.Disconnect()
	_, err = sshC1.ExecWithOutput(s.Context(), "kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	s.Require().NoError(err)
	// Delete the token file, as it shouldn't be needed after the controller has joined.
	_, err = sshC1.ExecWithOutput(s.Context(), "rm -f /etc/k0s.token")
	s.Require().NoError(err)
	s.NoError(s.InitController(1, "--token-file=/etc/k0s.token"))
	s.NoError(s.WaitJoinAPI(s.ControllerNode(1)))

	// Make one member leave the etcd cluster
	peerURL := membersFromJoined[s.ControllerNode(1)]
	s.makeNodeLeave(1, getHostnameFromURL(peerURL))
	refreshedMembers := s.getMembers(0)
	s.Len(refreshedMembers, 1)
	s.Contains(refreshedMembers, s.ControllerNode(0))

}

func TestHAControlplaneSuite(t *testing.T) {
	s := HAControlplaneSuite{
		common.BootlooseSuite{
			ControllerCount: 2,
		},
	}

	suite.Run(t, &s)

}

func getHostnameFromURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	hostName, _, _ := net.SplitHostPort(u.Host)
	return hostName
}
