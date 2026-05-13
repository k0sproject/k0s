// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcdlearner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/stretchr/testify/suite"
)

// EtcdLearnerSuite covers the join-as-learner safety property: a peer
// added through the k0s join API with an unreachable peer URL must not
// break quorum on the existing cluster, and the reconciler must not
// promote it (since it can never catch up). This stands in for the
// wrong-interface scenario the learner-join change exists to fix.
type EtcdLearnerSuite struct {
	common.BootlooseSuite
}

const phantomPeerURL = "https://192.0.2.42:2380" // RFC 5737 TEST-NET-1, unrouteable

func (s *EtcdLearnerSuite) TestUnreachableLearnerPreservesQuorum() {
	ctx := s.Context()

	// Boot controller0 as a single-node cluster.
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))
	s.Require().NoError(s.InitController(0, ""))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Baseline write on the single-node cluster.
	_, err = kc.CoreV1().ConfigMaps("default").Create(ctx,
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "pre-phantom"}},
		metav1.CreateOptions{})
	s.Require().NoError(err, "single-node cluster must accept writes before any peer is added")

	// Register a phantom peer through the k0s join API. The phantom's
	// advertised peer URL is TEST-NET-1 (RFC 5737), guaranteed
	// unrouteable. With the join-as-learner change, the existing cluster
	// adds it as a non-voting learner; quorum stays at 1.
	s.callJoinEtcd(ctx, s.ControllerNode(0), v1beta1.EtcdRequest{
		Node:        "phantom",
		PeerAddress: phantomPeerURL,
	})

	// Wait until the phantom shows up in the member list as a learner.
	s.Require().Eventually(func() bool {
		for _, m := range s.listMembers(ctx, s.ControllerNode(0)) {
			if matchesPhantom(m) {
				return m.IsLearner
			}
		}
		return false
	}, time.Minute, 2*time.Second, "phantom never appeared in etcd member list as learner")

	// The critical assertion: quorum stays at 1 and writes succeed.
	// Pre-fix, the join API added members as voters; an unreachable
	// voter would have made this write block until the request timeout.
	ctxWrite, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = kc.CoreV1().ConfigMaps("default").Create(ctxWrite,
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "post-phantom"}},
		metav1.CreateOptions{})
	s.Require().NoErrorf(err,
		"etcd quorum was lost after adding an unreachable peer; "+
			"the new member must enter as a non-voting learner")

	// Hold for a window and confirm the leader's reconciler does not
	// wrongly promote a peer that cannot have caught up. The reconciler
	// re-ticks every 30s on stuck learners; observing it stay False over
	// ~30s rules out a one-shot race.
	s.T().Log("verifying phantom stays a learner over a 30s observation window")
	hold, cancelHold := context.WithTimeout(ctx, 30*time.Second)
	defer cancelHold()
	for {
		select {
		case <-hold.Done():
			return
		case <-time.After(5 * time.Second):
		}
		var found bool
		for _, m := range s.listMembers(ctx, s.ControllerNode(0)) {
			if !matchesPhantom(m) {
				continue
			}
			found = true
			s.Require().Truef(m.IsLearner,
				"phantom was promoted to voter despite being unreachable; "+
					"reconciler must keep unreachable learners as learners")
		}
		s.Require().True(found, "phantom disappeared from etcd member list mid-test")
	}
}

// etcdMember is a minimal subset of etcdctl's member-list -w json output.
type etcdMember struct {
	ID        uint64   `json:"ID"`
	Name      string   `json:"name"`
	PeerURLs  []string `json:"peerURLs"`
	IsLearner bool     `json:"isLearner"`
}

// matchesPhantom returns true if m is the unreachable learner we added.
// etcd only records a member's self-reported Name after it publishes via
// raft, so an unreachable peer keeps an empty Name forever — match by
// peer URL instead.
func matchesPhantom(m etcdMember) bool {
	for _, u := range m.PeerURLs {
		if u == phantomPeerURL {
			return true
		}
	}
	return false
}

func (s *EtcdLearnerSuite) listMembers(ctx context.Context, node string) []etcdMember {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput(ctx,
		"/opt/etcdctl member list -w json "+
			"--endpoints=https://127.0.0.1:2379 "+
			"--cacert=/var/lib/k0s/pki/etcd/ca.crt "+
			"--cert=/var/lib/k0s/pki/apiserver-etcd-client.crt "+
			"--key=/var/lib/k0s/pki/apiserver-etcd-client.key")
	s.Require().NoErrorf(err, "etcdctl member list failed: %s", out)

	var resp struct {
		Members []etcdMember `json:"members"`
	}
	s.Require().NoError(json.Unmarshal([]byte(out), &resp), "etcdctl member list JSON: %s", out)
	return resp.Members
}

// callJoinEtcd uses the real k0s JoinClient to POST an EtcdRequest to the
// join API on the given node. The kubeconfig embedded in the join token
// points to the controller's container-internal IP, which is not
// reachable from the test host. We rewrite the server URL to the
// host-mapped port (localhost is in the k0s-api cert's SAN list, so TLS
// still validates) and hand the re-encoded token to JoinClientFromToken,
// so the rest of the code path — including the EtcdRequest encoding
// inside JoinClient.JoinEtcd — runs exactly as it does in production.
func (s *EtcdLearnerSuite) callJoinEtcd(ctx context.Context, node string, req v1beta1.EtcdRequest) {
	encToken, err := s.GetJoinToken("controller")
	s.Require().NoError(err)

	rawKubeconfig, err := token.DecodeJoinToken(encToken)
	s.Require().NoError(err)
	cfg, err := clientcmd.Load(rawKubeconfig)
	s.Require().NoError(err)

	m, err := s.MachineForName(node)
	s.Require().NoError(err)
	hostPort, err := m.HostPort(s.K0sAPIExternalPort)
	s.Require().NoError(err)
	hostURL := fmt.Sprintf("https://localhost:%d", hostPort)

	for _, cluster := range cfg.Clusters {
		cluster.Server = hostURL
	}

	rewritten, err := clientcmd.Write(*cfg)
	s.Require().NoError(err)
	reencoded, err := token.JoinEncode(bytes.NewReader(rewritten))
	s.Require().NoError(err)

	client, err := token.JoinClientFromToken(reencoded)
	s.Require().NoError(err)

	resp, err := client.JoinEtcd(ctx, req)
	s.Require().NoError(err)
	s.Require().NotEmpty(resp.InitialCluster, "join API returned no initial-cluster")
}

func TestEtcdLearnerSuite(t *testing.T) {
	s := EtcdLearnerSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}
	suite.Run(t, &s)
}
