/*
Copyright 2021 k0s authors

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
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type HAControlplaneSuite struct {
	common.FootlooseSuite
}

func (s *HAControlplaneSuite) getMembers(fromControllerIdx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(s.ControllerNode(fromControllerIdx))
	s.NoError(err)
	output, err := sshCon.ExecWithOutput("k0s etcd member-list")
	output = lastLine(output)
	s.NoError(err)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.NoError(json.Unmarshal([]byte(output), &members))
	return members.Members
}

func (s *HAControlplaneSuite) makeNodeLeave(executeOnControllerIdx int, peerAddress string) {
	sshCon, err := s.SSH(s.ControllerNode(executeOnControllerIdx))
	s.NoError(err)
	for i := 0; i < 20; i++ {
		_, err = sshCon.ExecWithOutput(fmt.Sprintf("k0s etcd leave %s", peerAddress))
		if err == nil {
			break
		}
		s.T().Logf("retrying k0s etcd leave...")
		time.Sleep(500 * time.Millisecond)
	}
	s.NoError(err)
}

func (s *HAControlplaneSuite) getCa(controllerIdx int) string {
	sshCon, err := s.SSH(s.ControllerNode(controllerIdx))
	s.NoError(err)
	ca, err := sshCon.ExecWithOutput("cat /var/lib/k0s/pki/ca.crt")
	s.NoError(err)

	return ca
}

func (s *HAControlplaneSuite) TestDeregistration() {
	s.NoError(s.InitController(0))
	token, err := s.GetJoinToken("controller", "")
	s.NoError(err)
	s.NoError(s.InitController(1, token))

	ca0 := s.getCa(0)
	s.Contains(ca0, "-----BEGIN CERTIFICATE-----")

	ca1 := s.getCa(1)
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
	sshC1, err := s.SSH(s.ControllerNode(1))
	s.Require().NoError(err)
	_, err = sshC1.ExecWithOutput("kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	s.Require().NoError(err)
	s.NoError(s.InitController(1, token))

	// Make one member leave the etcd cluster
	s.makeNodeLeave(1, membersFromJoined[s.ControllerNode(1)])
	refreshedMembers := s.getMembers(0)
	s.Len(refreshedMembers, 1)
	s.Contains(refreshedMembers, s.ControllerNode(0))

}

func TestHAControlplaneSuite(t *testing.T) {

	s := HAControlplaneSuite{
		common.FootlooseSuite{
			ControllerCount: 2,
		},
	}

	suite.Run(t, &s)

}

func lastLine(text string) string {
	if text == "" {
		return ""
	}
	parts := strings.Split(text, "\n")
	return parts[len(parts)-1]
}
