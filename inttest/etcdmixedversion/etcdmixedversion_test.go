// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package etcdmixedversion covers etcd membership changes in a control plane
// that runs two k0s versions at once, as it does in the middle of an upgrade
// that replaces controllers instead of upgrading them in place (Cluster API,
// k0smotron's Recreate strategy).
//
// The k0s version to upgrade from is taken from K0S_UPDATE_FROM_PATH, the one
// to upgrade to from K0S_PATH. Controllers that learned to join new members as
// etcd learners and to leave the etcd cluster by themselves must not rely on
// older controllers doing either.
package etcdmixedversion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/suite"
)

type binary string

const (
	// The k0s version upgraded from (K0S_UPDATE_FROM_PATH), seeded into
	// /usr/local/bin/k0s when the machines boot.
	oldK0s binary = "/dist/k0s"
	// The k0s version upgraded to (K0S_PATH).
	newK0s binary = "/dist/k0s-new"
)

const leaderLeaseName = "k0s-endpoint-reconciler"

type mixedVersionSuite struct {
	common.BootlooseSuite
}

// A new controller joins through the k0s API of an upgraded controller while
// an old controller is still the leader. The old leader never promotes etcd
// learners, so the new controller must join as a voting member.
//
// This is the second replacement of a Recreate rollout: the first new
// controller joined through an old one, and the control plane load balancer
// now routes the next join to that new controller.
type JoinUnderOldLeaderSuite struct{ mixedVersionSuite }

func (s *JoinUnderOldLeaderSuite) TestNewControllerBecomesVoter() {
	ctx := s.Context()

	s.T().Log("Starting controller0 on the old k0s; it becomes the leader")
	s.startController(ctx, 0, oldK0s, "")
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	s.Require().Equal(s.ControllerNode(0), s.leaderNode(ctx), "controller0 should hold the leader lease")

	s.T().Log("Joining controller1 on the new k0s through controller0's k0s API")
	s.startController(ctx, 1, newK0s, s.joinToken(ctx, 0))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(1)))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(1)))
	s.waitForVoter(ctx, s.ControllerNode(1), 3*time.Minute)

	s.T().Log("Joining controller2 on the new k0s through controller1's k0s API")
	s.startController(ctx, 2, newK0s, s.joinToken(ctx, 1))
	// The scenario only holds while the old controller is still leading.
	s.Require().Equal(s.ControllerNode(0), s.leaderNode(ctx), "controller0 should still hold the leader lease")
	s.waitForVoter(ctx, s.ControllerNode(2), 3*time.Minute)
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(2)))
	s.Require().Equal(s.ControllerNode(0), s.leaderNode(ctx), "controller0 should still hold the leader lease")
}

func TestJoinUnderOldLeaderSuite(t *testing.T) {
	requireUpdateFromPath(t)
	suite.Run(t, &JoinUnderOldLeaderSuite{mixedVersionSuite{common.BootlooseSuite{
		ControllerCount: 3,
		LaunchMode:      common.LaunchModeOpenRC,
	}}})
}

// An old controller is asked to leave while an upgraded controller leads. Old
// controllers stop without removing themselves from the etcd cluster, so the
// leader must remove it as soon as it is gone instead of waiting for it.
//
// This is how k0smotron retires an old controller in a Recreate rollout once
// the leader lease has moved to an upgraded controller.
type OldControllerLeavesSuite struct{ mixedVersionSuite }

func (s *OldControllerLeavesSuite) TestOldControllerIsRemovedPromptly() {
	ctx := s.Context()

	s.T().Log("Starting controller0 on the new k0s; it becomes the leader")
	s.startController(ctx, 0, newK0s, "")
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	token := s.joinToken(ctx, 0)

	for idx, bin := range []binary{1: oldK0s, 2: newK0s} {
		if idx == 0 {
			continue
		}
		s.T().Logf("Joining %s on %s", s.ControllerNode(idx), bin)
		s.startController(ctx, idx, bin, token)
		s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(idx)))
		s.waitForVoter(ctx, s.ControllerNode(idx), 3*time.Minute)
	}
	s.Require().Equal(s.ControllerNode(0), s.leaderNode(ctx), "controller0 should hold the leader lease")

	leaving := s.ControllerNode(1)
	s.waitForEtcdMemberObject(ctx, leaving)

	s.T().Logf("Marking %s (old k0s) to leave", leaving)
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	start := time.Now()
	s.Require().NoError(kc.RESTClient().Patch(types.MergePatchType).
		AbsPath("/apis/etcd.k0sproject.io/v1beta1/etcdmembers/" + leaving).
		Body([]byte(`{"spec":{"leave":true}}`)).
		Do(ctx).Error())

	// The leader asks the old controller to shut down; wait for it to do so,
	// then make sure it stays down, as it would when its machine is deleted.
	s.Require().NoError(common.Poll(ctx, func(ctx context.Context) (bool, error) {
		lease, err := kc.CoordinationV1().Leases(corev1.NamespaceNodeLease).Get(ctx, "k0s-ctrl-"+leaving, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return !kubeutil.IsValidLease(*lease), nil
	}), "%s didn't shut down", leaving)
	s.T().Logf("%s shut down after %s", leaving, time.Since(start).Round(time.Second))
	s.stopController(ctx, leaving)

	// Without the fix, the leader waits 5 minutes for a self-leave that never
	// comes. Allow a generous margin below that, but watch for longer so that
	// the log shows how long the removal actually took.
	const deadline = 2 * time.Minute
	var removedAfter time.Duration
	s.Require().NoError(wait.PollUntilContextTimeout(ctx, 2*time.Second, 7*time.Minute, true, func(ctx context.Context) (bool, error) {
		members, err := s.etcdMembers(ctx, 0)
		if err != nil {
			s.T().Log("Failed to list etcd members, retrying: ", err)
			return false, nil
		}
		if slices.ContainsFunc(members, func(m etcdMember) bool { return m.Name == leaving }) {
			return false, nil
		}
		removedAfter = time.Since(start)
		return true, nil
	}), "%s was never removed from the etcd cluster", leaving)

	s.T().Logf("%s was removed from the etcd cluster %s after it was marked to leave", leaving, removedAfter.Round(time.Second))
	s.Less(removedAfter, deadline, "the leader waited for an old controller to leave by itself")
}

func TestOldControllerLeavesSuite(t *testing.T) {
	requireUpdateFromPath(t)
	suite.Run(t, &OldControllerLeavesSuite{mixedVersionSuite{common.BootlooseSuite{
		ControllerCount: 3,
		LaunchMode:      common.LaunchModeOpenRC,
	}}})
}

func requireUpdateFromPath(t *testing.T) {
	if os.Getenv("K0S_UPDATE_FROM_PATH") == "" {
		t.Skip("K0S_UPDATE_FROM_PATH isn't set; this test needs a k0s executable to upgrade from")
	}
}

// Starts a k0s controller on the given machine with the given k0s executable,
// without waiting for it to become ready: a controller that never gets a vote
// in etcd never does.
func (s *mixedVersionSuite) startController(ctx context.Context, idx int, bin binary, token string) {
	node := s.ControllerNode(idx)
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	for _, cmd := range []string{
		"install -m 0755 " + string(bin) + " /usr/local/bin/k0s",
		"/usr/local/bin/k0s version",
		"/usr/local/bin/k0s install controller",
		fmt.Sprintf(`sed -i 's#^command_args=.*$#command_args="controller --debug %s"#g' /etc/init.d/k0scontroller`, token),
		"/etc/init.d/k0scontroller start",
	} {
		out, err := ssh.ExecWithOutput(ctx, cmd)
		s.Require().NoError(err, "%s: %s: %s", node, cmd, out)
		if strings.HasSuffix(cmd, " version") {
			s.T().Logf("%s runs k0s %s", node, out)
		}
	}
}

func (s *mixedVersionSuite) stopController(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	// The controller may have stopped already.
	out, err := ssh.ExecWithOutput(ctx, "/etc/init.d/k0scontroller stop || true; /etc/init.d/k0scontroller zap || true")
	s.T().Logf("Stopped k0s on %s: %v: %s", node, err, out)
}

// Creates a controller join token on the given controller. It points to that
// controller's k0s API, so that controller serves the join.
func (s *mixedVersionSuite) joinToken(ctx context.Context, idx int) string {
	ssh, err := s.SSH(ctx, s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	token, err := ssh.ExecWithOutput(ctx, "/usr/local/bin/k0s token create --role=controller")
	s.Require().NoError(err, token)
	return strings.TrimSpace(token)
}

// Returns the name of the controller that holds the leader lease.
func (s *mixedVersionSuite) leaderNode(ctx context.Context) string {
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	var leader string
	s.Require().NoError(common.Poll(ctx, func(ctx context.Context) (bool, error) {
		lease, err := kc.CoordinationV1().Leases(corev1.NamespaceNodeLease).Get(ctx, leaderLeaseName, metav1.GetOptions{})
		if err != nil || lease.Spec.HolderIdentity == nil {
			return false, nil
		}
		leases, err := kc.CoordinationV1().Leases(corev1.NamespaceNodeLease).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		for _, l := range leases.Items {
			if node, ok := strings.CutPrefix(l.Name, "k0s-ctrl-"); ok &&
				l.Spec.HolderIdentity != nil && *l.Spec.HolderIdentity == *lease.Spec.HolderIdentity {
				leader = node
				return true, nil
			}
		}
		return false, nil
	}))
	return leader
}

// Waits until the given controller is a voting member of the etcd cluster.
// Fails if it is still missing or a learner after the given timeout.
func (s *mixedVersionSuite) waitForVoter(ctx context.Context, node string, timeout time.Duration) {
	var last []etcdMember
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		members, err := s.etcdMembers(ctx, 0)
		if err != nil {
			s.T().Log("Failed to list etcd members, retrying: ", err)
			return false, nil
		}
		last = members
		idx := slices.IndexFunc(members, func(m etcdMember) bool { return m.Name == node })
		return idx >= 0 && !members[idx].IsLearner, nil
	})
	if err != nil {
		s.FailNowf("not a voting etcd member", "%s isn't a voting etcd member after %s; members: %s", node, timeout, formatMembers(last))
	}
	s.T().Logf("%s is a voting etcd member; members: %s", node, formatMembers(last))
}

func (s *mixedVersionSuite) waitForEtcdMemberObject(ctx context.Context, node string) {
	c, err := s.EtcdMemberClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(common.Poll(ctx, func(ctx context.Context) (bool, error) {
		_, err := c.EtcdMembers().Get(ctx, node, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return err == nil, nil
	}), "EtcdMember %s never appeared", node)
}

// A minimal subset of etcdctl's member list JSON output.
type etcdMember struct {
	ID        uint64 `json:"ID"`
	Name      string `json:"name"`
	IsLearner bool   `json:"isLearner"`
}

func formatMembers(members []etcdMember) string {
	var b strings.Builder
	for i, m := range members {
		if i > 0 {
			b.WriteString(", ")
		}
		role := "voter"
		if m.IsLearner {
			role = "learner"
		}
		name := m.Name
		if name == "" {
			name = "(unstarted)"
		}
		fmt.Fprintf(&b, "%s=%s", name, role)
	}
	return "[" + b.String() + "]"
}

// Lists the etcd members as seen by the given controller's etcd.
func (s *mixedVersionSuite) etcdMembers(ctx context.Context, idx int) ([]etcdMember, error) {
	ssh, err := s.SSH(ctx, s.ControllerNode(idx))
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := ssh.ExecWithOutput(ctx,
		"/opt/etcdctl member list -w json "+
			"--endpoints=https://127.0.0.1:2379 "+
			"--cacert=/var/lib/k0s/pki/etcd/ca.crt "+
			"--cert=/var/lib/k0s/pki/apiserver-etcd-client.crt "+
			"--key=/var/lib/k0s/pki/apiserver-etcd-client.key")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, out)
	}
	var resp struct {
		Members []etcdMember `json:"members"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, errors.Join(err, fmt.Errorf("etcdctl output: %s", out))
	}
	return resp.Members, nil
}
