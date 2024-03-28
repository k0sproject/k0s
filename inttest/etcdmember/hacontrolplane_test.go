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
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
)

type EtcdMemberSuite struct {
	common.BootlooseSuite
}

func (s *EtcdMemberSuite) getMembers(fromControllerIdx int) map[string]string {
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

func (s *EtcdMemberSuite) TestDeregistration() {

	var joinToken string
	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))
		s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(idx)))
		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		s.Require().Len(s.GetMembers(idx), s.BootlooseSuite.ControllerCount)
	}
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	basePath := "apis/etcd.k0sproject.io/v1beta1/etcdmembers/%s"

	// Check each node is present in the etcd cluster
	expectedObjects := []string{"controller0", "controller1", "controller2"}
	for i, obj := range expectedObjects {
		em := &etcdv1beta1.EtcdMember{}
		err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, obj)).Do(s.Context()).Into(em) //DoRaw(s.Context())
		s.Require().NoError(err)
		s.Require().Equal(em.PeerAddress, s.GetControllerIPAddress(i))
	}

	// Make one of the nodes leave
	path := fmt.Sprintf(basePath, "controller2")
	result := kc.RESTClient().Delete().AbsPath(path).Do(s.Context())
	s.Require().NoError(result.Error())

	// Check that the node is gone from the etcd cluster according to etcd
	members := s.getMembers(0)
	s.Require().Len(members, s.BootlooseSuite.ControllerCount-1)
	s.Require().NotContains(members, "controller2")
	// Make sure the EtcdMember CR is gone
	_, err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, "controller2")).DoRaw(s.Context())
	s.Require().Error(err)
	// Check that the error is NotFound
	s.Require().Contains(err.Error(), "the server could not find the requested resource")

	// Stop k0s and reset the node
	s.StopController(s.ControllerNode(2))
	s.ResetNode(s.ControllerNode(2))

	// Make the node rejoin
	s.Require().NoError(s.InitController(2, joinToken))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(2)))

	// Final sanity -- ensure all nodes see each other according to etcd
	members = s.getMembers(0)
	s.Require().Len(members, s.BootlooseSuite.ControllerCount)
	s.Require().Contains(members, "controller2")

	// Check the CR is present again
	em := &etcdv1beta1.EtcdMember{}
	err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, "controller2")).Do(s.Context()).Into(em)
	s.Require().NoError(err)
	s.Require().Equal(em.PeerAddress, s.GetControllerIPAddress(2))

	// Check that after restarting the controller, the member is still present
	s.Require().NoError(s.RestartController(s.ControllerNode(2)))
	em = &etcdv1beta1.EtcdMember{}
	err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, "controller2")).Do(s.Context()).Into(em)
	s.Require().NoError(err)
	s.Require().Equal(em.PeerAddress, s.GetControllerIPAddress(2))

}

func TestEtcdMemberSuite(t *testing.T) {
	s := EtcdMemberSuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}

	suite.Run(t, &s)

}
