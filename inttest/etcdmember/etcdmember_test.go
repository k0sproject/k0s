/*
Copyright 2024 k0s authors

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
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
)

type EtcdMemberSuite struct {
	common.BootlooseSuite
}

func (s *EtcdMemberSuite) getMembers(ctx context.Context, fromControllerIdx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(ctx, s.ControllerNode(fromControllerIdx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	output, err := sshCon.ExecWithOutput(ctx, "/usr/local/bin/k0s etcd member-list 2>/dev/null")
	s.T().Logf("k0s etcd member-list output: %s", output)
	s.Require().NoError(err)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.NoError(json.Unmarshal([]byte(output), &members))
	return members.Members
}

func (s *EtcdMemberSuite) TestDeregistration() {
	ctx := s.Context()
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

	etcdMemberClient, err := s.EtcdMemberClient(s.ControllerNode(0))

	// Check each node is present in the etcd cluster and reports joined state
	expectedObjects := []string{"controller0", "controller1", "controller2"}

	// Use errorgroup to wait for all the statuses to be updated
	eg := errgroup.Group{}

	for i, obj := range expectedObjects {
		i, obj := i, obj

		eg.Go(func() error {
			s.T().Logf("verifying initial status of %s", obj)
			em := &etcdv1beta1.EtcdMember{}

			err = watch.EtcdMembers(etcdMemberClient.EtcdMembers()).
				WithObjectName(obj).
				WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
				Until(ctx, func(item *etcdv1beta1.EtcdMember) (done bool, err error) {
					c := item.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
					if c != nil {
						// We have the condition so we can bail out
						em = item
					}
					return c != nil, nil
				})

			// We've got the condition, verify status details
			if err != nil {
				return err
			}
			if em.Status.PeerAddress != s.GetControllerIPAddress(i) {
				return fmt.Errorf("expected PeerAddress %s, got %s", s.GetControllerIPAddress(i), em.Status.PeerAddress)
			}

			c := em.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
			if c == nil {
				return fmt.Errorf("expected condition %s, got nil", etcdv1beta1.ConditionTypeJoined)
			}
			if c.Status != etcdv1beta1.ConditionTrue {
				return fmt.Errorf("expected condition %s to be %s, got %s", etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionTrue, c.Status)
			}
			return nil
		})

	}

	s.T().Log("waiting to see correct statuses on EtcdMembers")
	s.NoError(eg.Wait())
	s.T().Log("All statuses found")
	// Make one of the nodes leave
	s.leaveNode(ctx, "controller2")

	// Check that the node is gone from the etcd cluster according to etcd itself
	members := s.getMembers(ctx, 0)
	s.Require().Len(members, s.BootlooseSuite.ControllerCount-1)
	s.Require().NotContains(members, "controller2")

	// Make sure the EtcdMember CR status is successfully updated
	em := s.getMember(ctx, "controller2")
	s.Require().Equal(em.Status.ReconcileStatus, "Success")
	s.Require().Equal(etcdv1beta1.ConditionFalse, em.Status.GetCondition(etcdv1beta1.ConditionTypeJoined).Status)

	// Stop k0s and reset the node
	s.Require().NoError(s.StopController(s.ControllerNode(2)))
	s.Require().NoError(common.ResetNode(s.ControllerNode(2), &s.BootlooseSuite))

	// Make the node rejoin
	s.Require().NoError(s.InitController(2, joinToken))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(2)))

	// Final sanity -- ensure all nodes see each other according to etcd
	members = s.getMembers(ctx, 0)
	s.Require().Len(members, s.BootlooseSuite.ControllerCount)
	s.Require().Contains(members, "controller2")

	// Check the CR is present again
	em = s.getMember(ctx, "controller2")
	s.Require().Equal(em.Status.PeerAddress, s.GetControllerIPAddress(2))
	s.Require().Equal(false, em.Spec.Leave)
	s.Require().Equal(etcdv1beta1.ConditionTrue, em.Status.GetCondition(etcdv1beta1.ConditionTypeJoined).Status)

	// Check that after restarting the controller, the member is still present
	s.Require().NoError(s.RestartController(s.ControllerNode(2)))
	em = &etcdv1beta1.EtcdMember{}
	err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, "controller2")).Do(ctx).Into(em)
	s.Require().NoError(err)
	s.Require().Equal(em.Status.PeerAddress, s.GetControllerIPAddress(2))

}

const basePath = "apis/etcd.k0sproject.io/v1beta1/etcdmembers/%s"

func (s *EtcdMemberSuite) leaveNode(ctx context.Context, name string) {
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Patch the EtcdMember CR to set the Leave flag
	path := fmt.Sprintf(basePath, name)
	patch := []byte(`{"spec":{"leave":true}}`)
	result := kc.RESTClient().Patch("application/merge-patch+json").AbsPath(path).Body(patch).Do(ctx)

	s.Require().NoError(result.Error())
	s.T().Logf("marked %s for leaving, waiting to see the state updated", name)
	err = common.Poll(ctx, func(ctx context.Context) (done bool, err error) {
		em := &etcdv1beta1.EtcdMember{}
		err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, name)).Do(ctx).Into(em)
		s.Require().NoError(err)

		c := em.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
		if c == nil {
			return false, nil
		}
		s.T().Logf("JoinStatus = %s, waiting for %s", c.Status, etcdv1beta1.ConditionFalse)
		return c.Status == etcdv1beta1.ConditionFalse, nil

	})
	s.Require().NoError(err)

}

// getMember returns the etcd member CR for the given name
func (s *EtcdMemberSuite) getMember(ctx context.Context, name string) *etcdv1beta1.EtcdMember {
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	em := &etcdv1beta1.EtcdMember{}
	err = kc.RESTClient().Get().AbsPath(fmt.Sprintf(basePath, name)).Do(ctx).Into(em)
	s.Require().NoError(err)
	return em
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
