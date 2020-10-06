package etcd

import (
	"encoding/json"
	"fmt"
	"github.com/Mirantis/mke/inttest/common"
	"github.com/stretchr/testify/suite"
	"strings"
	"testing"
)

type EtcdSuite struct {
	common.FootlooseSuite
}

func (s *EtcdSuite) getMembers(fromControllerIdx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	node := fmt.Sprintf("controller%d", fromControllerIdx)
	sshCon, err := s.SSH(node)
	s.NoError(err)
	output, err := sshCon.ExecWithOutput("mke etcd member-list")
	output = lastLine(output)
	s.NoError(err)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.NoError(json.Unmarshal([]byte(output), &members))
	return members.Members
}

func (s *EtcdSuite) makeNodeLeave(executeOnControllerIdx int, peerAddress string) {
	node := fmt.Sprintf("controller%d", executeOnControllerIdx)
	sshCon, err := s.SSH(node)
	s.NoError(err)
	_, err = sshCon.ExecWithOutput(fmt.Sprintf("mke etcd leave %s", peerAddress))
	s.NoError(err)
}

func (s *EtcdSuite) TestDeregistration() {
	s.NoError(s.InitMainController())
	token, err := s.GetJoinToken("controller")
	s.NoError(err)
	s.NoError(s.JoinController(1, token))
	membersFromMain := s.getMembers(0)
	membersFromJoined := s.getMembers(1)
	s.Equal(membersFromMain, membersFromJoined,
		"etcd cluster members list must be the same across all nodes")
	s.Len(membersFromJoined, 2, "etcd cluster must have exactly 2 members")
	s.makeNodeLeave(1, membersFromJoined["controller1"])
	refreshedMembers := s.getMembers(0)
	s.Len(refreshedMembers, 1)
	s.Contains(refreshedMembers, "controller0")
}

func TestEtcdSuite(t *testing.T) {

	s := EtcdSuite{
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
