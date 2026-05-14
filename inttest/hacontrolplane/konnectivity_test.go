// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package hacontrolplane

import (
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/stretchr/testify/suite"
)

// KonnectivityLeaseSuite verifies that konnectivity server count is delegated
// to the lease controller (k0s issue #5736): the agent DaemonSet must no
// longer be re-rendered when the controller count changes, and the server/agent
// must run with the new lease-controller flags + RBAC.
type KonnectivityLeaseSuite struct {
	common.BootlooseSuite
}

func (s *KonnectivityLeaseSuite) TestLeaseCounting() {
	ctx := s.Context()

	// Bring up 3 controllers + 1 worker. Three so etcd retains quorum when
	// we kill one controller later — a 2-node etcd loses quorum on any
	// failure, which would freeze the apiserver and make every assertion
	// below hang.
	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	for i := 1; i <= 2; i++ {
		s.PutFile(s.ControllerNode(i), "/etc/k0s.token", token)
		s.Require().NoError(s.InitController(i, "--token-file=/etc/k0s.token"))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(i)))
	}

	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	// 1. RBAC for the konnectivity-agent (read leases) is in place.
	_, err = kc.RbacV1().ClusterRoles().
		Get(ctx, "system:konnectivity-agent", metav1.GetOptions{})
	s.NoError(err, "ClusterRole system:konnectivity-agent must exist")
	_, err = kc.RbacV1().ClusterRoleBindings().
		Get(ctx, "system:konnectivity-agent", metav1.GetOptions{})
	s.NoError(err, "ClusterRoleBinding system:konnectivity-agent must exist")

	// 3. Konnectivity-server runs with --enable-lease-controller and no
	//    longer takes a static --server-count.
	srvArgs, err := s.execOnNode(s.ControllerNode(0),
		"cat /proc/$(pidof konnectivity-server)/cmdline | tr '\\0' ' '")
	s.Require().NoError(err)
	s.Contains(srvArgs, "--enable-lease-controller")
	s.NotContains(srvArgs, "--server-count=")

	// 4. Konnectivity-agent DaemonSet runs with --count-server-leases=true and
	//    no longer carries the K0S_CONTROLLER_COUNT trigger env var.
	//    Worker-Ready doesn't guarantee the manifest applier has pushed the DS
	//    yet, so wait for it to appear.
	var ds *appsv1.DaemonSet
	s.Require().Eventually(func() bool {
		got, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).
			Get(ctx, "konnectivity-agent", metav1.GetOptions{})
		if err != nil {
			return false
		}
		ds = got
		return true
	}, 2*time.Minute, 2*time.Second, "konnectivity-agent DaemonSet never appeared")
	s.Require().Len(ds.Spec.Template.Spec.Containers, 1)
	c := ds.Spec.Template.Spec.Containers[0]
	s.Contains(c.Args, "--count-server-leases=true")
	for _, e := range c.Env {
		s.NotEqual("K0S_CONTROLLER_COUNT", e.Name,
			"K0S_CONTROLLER_COUNT env var must be gone — it only existed to force agent restarts")
	}

	// 5. All three konnectivity-server instances publish a lease under
	//    k8s-app=konnectivity-server in kube-system. This proves the lease
	//    controller is actually running — upstream names them
	//    "konnectivity-proxy-server-<serverID>" with the default
	//    --lease-label=k8s-app=konnectivity-server.
	s.Require().Eventually(func() bool {
		return s.countServerLeases(kc) == 3
	}, 90*time.Second, 2*time.Second, "expected 3 konnectivity-server leases")

	// Also wait until the agent DS is fully Ready so the generation below
	// reflects the settled state, not a mid-rollout glimpse.
	s.waitDSReady(kc, "konnectivity-agent")
	genBefore := s.getAgentDS(kc).Generation
	s.T().Logf("konnectivity-agent DaemonSet generation before controller leaves: %d", genBefore)

	// 6. THE behavioral assertion. Stop one controller; the agent DaemonSet
	//    must NOT be re-rendered. Pre-refactor, k0s rewrote the manifest with
	//    a new K0S_CONTROLLER_COUNT value, bumping .metadata.generation and
	//    rolling every agent pod. With lease-counting, the surviving server
	//    just sees one fewer lease and the agent reacts on its own.
	_, err = s.execOnNode(s.ControllerNode(1),
		"kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	s.Require().NoError(err)

	// The dead server's lease expires after LeaseDuration=30s and is GC'd on
	// the next GCInterval=15s tick — so ~45s worst case. Assert positively
	// that the count drops to 2, which proves the mechanism works end-to-end.
	s.Require().Eventually(func() bool {
		return s.countServerLeases(kc) == 2
	}, 90*time.Second, 2*time.Second, "dead server's lease was never GC'd")

	// And the regression guard: the surviving controller must NOT have
	// re-rendered the agent DS in response to its peer going away.
	dsAfter := s.getAgentDS(kc)
	s.Equal(genBefore, dsAfter.Generation,
		"konnectivity-agent DaemonSet generation must not change when controller count changes")
}

func (s *KonnectivityLeaseSuite) countServerLeases(kc *kubernetes.Clientset) int {
	ll, err := kc.CoordinationV1().Leases(metav1.NamespaceSystem).
		List(s.Context(), metav1.ListOptions{LabelSelector: "k8s-app=konnectivity-server"})
	if err != nil {
		s.T().Logf("list konnectivity-server leases: %v", err)
		return -1
	}
	return len(ll.Items)
}

func (s *KonnectivityLeaseSuite) getAgentDS(kc *kubernetes.Clientset) *appsv1.DaemonSet {
	ds, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).
		Get(s.Context(), "konnectivity-agent", metav1.GetOptions{})
	s.Require().NoError(err)
	return ds
}

func (s *KonnectivityLeaseSuite) waitDSReady(kc *kubernetes.Clientset, name string) {
	s.Require().Eventually(func() bool {
		ds, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).
			Get(s.Context(), name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				s.T().Logf("get %s: %v", name, err)
			}
			return false
		}
		return ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled
	}, 3*time.Minute, 5*time.Second, "DaemonSet %s never became fully Ready", name)

	// Sanity: at least one agent pod exists and is Ready.
	pods, err := kc.CoreV1().Pods(metav1.NamespaceSystem).
		List(s.Context(), metav1.ListOptions{LabelSelector: "k8s-app=" + name})
	s.Require().NoError(err)
	s.Require().NotEmpty(pods.Items)
	for _, p := range pods.Items {
		ready := false
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		s.True(ready, "agent pod %s should be Ready", p.Name)
	}
}

func (s *KonnectivityLeaseSuite) execOnNode(node, cmd string) (string, error) {
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()
	return ssh.ExecWithOutput(s.Context(), cmd)
}

func TestKonnectivityLeaseSuite(t *testing.T) {
	s := KonnectivityLeaseSuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
