// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplbbgp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/inttest/common/ociimages"
	"github.com/k0sproject/k0s/pkg/token"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cplbBGPSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
}

const controllerConfig = `
spec:
  images:
    default_pull_policy: IfNotPresent
  api:
    externalAddress: {{ .VIP }}
  network:
{{- if .IPv6 }}
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
{{- end }}
    controlPlaneLoadBalancing:
      enabled: true
      type: BGP
      bgp:
        virtualIPs: ["{{ .VIP }}/{{ .Bits }}"{{ if .IPv6 }}, "10.254.254.20/32"{{ end }}]
        communities: ["65000:100", "65000:1:100"]
        as: {{ .AS }}
{{- if .RouterID }}
        routerID: {{ .RouterID }}
{{- end }}
{{- if .IPv6 }}
        peers:
        - address: {{ .Router }}
          as: 65000
          extendedNextHop: true
          bfd: {}
{{- else }}
        listen:
          port: 1790
          neighbors:
          - prefix: {{ .RouterPrefix }}
            as: 65000
            password: k0s-test
{{- end }}
`

const routerConfig = `frr defaults datacenter
{{- if eq .Family "ipv6" }}
interface eth0
 ipv6 nd prefix {{ .Prefix }} no-autoconfig
exit
{{- end }}
router bgp 65000
 bgp router-id 192.0.2.254
 no bgp ebgp-requires-policy
 bgp bestpath as-path multipath-relax
 neighbor controllers peer-group
 neighbor controllers remote-as {{ .AS }}
 neighbor controllers capability extended-nexthop
{{- if eq .Family "ipv6" }}
 neighbor controllers passive
 neighbor controllers bfd
{{- else }}
 neighbor controllers password k0s-test
{{- end }}
{{- range .Controllers }}
 neighbor {{ . }} peer-group controllers
{{- if ne $.Family "ipv6" }}
 neighbor {{ . }} port 1790
{{- end }}
{{- end }}
 address-family {{ .Family }} unicast
  neighbor controllers activate
  maximum-paths 8
  maximum-paths ibgp 8
 exit-address-family
{{- if eq .Family "ipv6" }}
 address-family ipv4 unicast
  neighbor controllers activate
  maximum-paths 8
 exit-address-family
{{- end }}
`

func (s *cplbBGPSuite) address(node string) string {
	if s.isIPv6Only {
		return common.FirstPublicIPv6Address(&s.BootlooseSuite, node)
	}
	return s.GetIPAddress(node)
}

func (s *cplbBGPSuite) TestK0sGetsUp() {
	ctx := s.Context()

	// IPv4 peers over iBGP and IPv6 over eBGP, so that both are covered.
	vip, bits, as, family, ipCmd := "10.254.254.10", 32, 65000, "ipv4", "ip"
	if s.isIPv6Only {
		vip, bits, as, family, ipCmd = "fd99::10", 128, 65001, "ipv6", "ip -6"
	}
	// The first worker machine doesn't run k0s, it's the BGP router of the controllers.
	router := s.WorkerNode(0)
	routerAddress := s.address(router)

	controllers := make([]string, 0, s.ControllerCount)
	nodes := append(make([]string, 0, 1+s.ControllerCount), s.WorkerNode(1))
	for idx := range s.ControllerCount {
		controllers = append(controllers, s.address(s.ControllerNode(idx)))
		nodes = append(nodes, s.ControllerNode(idx))
	}
	// The extended next hop capability makes FRR send router advertisements, which mustn't add addresses to the nodes.
	routerIP, err := netip.ParseAddr(routerAddress)
	s.Require().NoError(err)
	s.PutFileTemplate(router, "/etc/frr/frr.conf", routerConfig, map[string]any{
		"AS": as, "Controllers": controllers, "Family": family, "Prefix": netip.PrefixFrom(routerIP, 64).Masked(),
	})
	s.exec(ctx, router, strings.Join([]string{
		"sed -i -e s/^bgpd=no/bgpd=yes/ -e s/^bfdd=no/bfdd=yes/ /etc/frr/daemons",
		"sysctl -w net.ipv4.fib_multipath_hash_policy=1 net.ipv6.fib_multipath_hash_policy=1",
		"sysctl -w net.ipv4.ip_forward=1 net.ipv6.conf.all.forwarding=1 net.ipv4.conf.all.send_redirects=0 net.ipv4.conf.eth0.send_redirects=0",
		"rc-service frr start </dev/null >/dev/null 2>&1",
	}, " && "))

	// The nodes don't learn routes from the router, and the join tokens point at the VIP.
	for _, node := range nodes {
		s.exec(ctx, node, fmt.Sprintf("%s route add %s/%d via %s && sysctl -w net.ipv4.conf.all.accept_redirects=0 net.ipv6.conf.all.accept_redirects=0", ipCmd, vip, bits, routerAddress))
	}

	featureGates := "--feature-gates=ControlPlaneLoadBalancingBGP=true"
	extraArgs := []string{}
	if s.isIPv6Only {
		featureGates += ",IPv6SingleStack=true"
		// There's no IPv6 connectivity to the outside world in the CI environment.
		extraArgs = append(extraArgs, "--disable-components=coredns")
	}

	var joinToken string
	for idx := range s.ControllerCount {
		routerID := ""
		if s.isIPv6Only {
			routerID = fmt.Sprintf("192.0.2.%d", 11+idx)
		}
		s.PutFileTemplate(s.ControllerNode(idx), "/tmp/k0s.yaml", controllerConfig, map[string]any{
			"VIP": vip, "Bits": bits, "AS": as, "RouterID": routerID, "Router": routerAddress, "IPv6": s.isIPv6Only,
			"RouterPrefix": netip.PrefixFrom(routerIP, 24).Masked(),
		})
		args := append([]string{"--config=/tmp/k0s.yaml", featureGates}, extraArgs...)
		s.Require().NoError(s.InitController(idx, append(args, joinToken)...))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))
		if idx == 0 {
			joinToken, err = s.GetJoinToken(token.RoleController)
			s.Require().NoError(err)
		}
	}

	s.T().Log("Waiting for every controller to advertise the VIP")
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, slices.Sorted(slices.Values(controllers)), s.nextHops(ctx, family, vip, bits), "Every controller should be a next hop of the VIP")
	}, 3*time.Minute, time.Second)

	s.T().Log("Testing that the VIP is only on lo, without a proxy")
	for idx := range s.ControllerCount {
		node := s.ControllerNode(idx)
		s.Contains(s.exec(ctx, node, "ip -o addr show dev lo"), fmt.Sprintf(" %s/%d ", vip, bits))
		s.NotContains(s.exec(ctx, node, "ip -o addr show dev eth0"), " "+vip+"/")
		s.NotContains(s.exec(ctx, node, "netstat -tln"), ":6444 ")
	}

	if s.isIPv6Only {
		s.T().Log("Waiting for the BFD sessions")
		s.Require().EventuallyWithT(func(c *assert.CollectT) {
			assert.Equal(c, strconv.Itoa(s.ControllerCount), strings.TrimSpace(s.exec(ctx, router, "vtysh -c 'show bfd peers brief' | grep -c ' up ' || true")))
		}, time.Minute, time.Second)
	}

	frozen := s.ControllerNode(1)
	s.T().Logf("Testing that %s withdraws its route while its API server isn't ready", frozen)
	s.exec(ctx, frozen, "pkill -STOP kube-apiserver")
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, slices.Sorted(slices.Values([]string{controllers[0], controllers[2]})), s.nextHops(ctx, family, vip, bits))
	}, time.Minute, time.Second)
	s.Contains(s.exec(ctx, router, fmt.Sprintf("vtysh -c 'show bgp neighbors %s json'", controllers[1])), `"bgpState":"Established"`,
		"The BGP session should stay up")
	s.NotContains(s.exec(ctx, frozen, "ip -o addr show dev lo"), " "+vip+"/")
	s.exec(ctx, frozen, "pkill -CONT kube-apiserver")
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, slices.Sorted(slices.Values(controllers)), s.nextHops(ctx, family, vip, bits))
	}, time.Minute, time.Second)

	s.T().Log("Testing that the VIP route carries the configured communities")
	route := s.exec(ctx, router, fmt.Sprintf("vtysh -c 'show bgp %s unicast %s/%d json'", family, vip, bits))
	s.Contains(route, `"65000:100"`, "The VIP route should carry the standard community")
	s.Contains(route, `"65000:1:100"`, "The VIP route should carry the large community")

	if s.isIPv6Only {
		s.T().Log("Waiting for every controller to advertise the IPv4 VIP over its IPv6 session")
		s.Require().EventuallyWithT(func(c *assert.CollectT) {
			assert.Equal(c, slices.Sorted(slices.Values(controllers)), s.nextHops(ctx, "ipv4", "10.254.254.20", 32),
				"Every controller should be an IPv6 next hop of the IPv4 VIP")
		}, time.Minute, time.Second)
	}

	s.T().Log("Testing that requests routed to the VIP reach every controller")
	// The second worker machine's connections are forwarded by the router, which hashes them with their ports.
	probe := fmt.Sprintf(`for i in $(seq 20); do c=$(curl -sk -m 5 -o /dev/null -w '%%{certs}' https://%s/healthz) && [ -n "$c" ] && echo "$c" | sha256sum; done | sort -u | wc -l`,
		net.JoinHostPort(vip, "6443"))
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, strconv.Itoa(s.ControllerCount), strings.TrimSpace(s.exec(ctx, s.WorkerNode(1), probe)),
			"Requests to the VIP should reach every controller")
	}, time.Minute, time.Second)

	s.T().Log("Joining a worker through the VIP")
	workerToken, err := s.GetJoinToken(token.RoleWorker)
	s.Require().NoError(err)
	s.Require().NoError(s.RunWithToken(s.WorkerNode(1), workerToken))

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	client, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(1), client))

	s.T().Log("Waiting for the konnectivity agent to connect to every controller through the VIP")
	s.Require().NoError(common.WaitForDaemonSet(ctx, client, "konnectivity-agent", metav1.NamespaceSystem))
	s.Require().NoError(common.VerifyKonnectivityMesh(ctx, restConfig, client, s.T(), uint(s.ControllerCount), 1))

	s.T().Log("Testing that the logs of a pod on the worker can be read through the VIP")
	alpine, err := ociimages.Alpine(ctx)
	s.Require().NoError(err)
	_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "logger"},
		Spec: corev1.PodSpec{
			NodeName: s.WorkerNode(1),
			Containers: []corev1.Container{{
				Name: "logger", Image: alpine, ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sh", "-c", "while true; do echo hello; sleep 1; done"},
			}},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, client, "logger", metav1.NamespaceDefault))
	s.PutFile(router, "/tmp/admin.conf", s.exec(ctx, s.ControllerNode(0), s.K0sFullPath+" kubeconfig admin"))
	for range 9 {
		s.Equal("hello", strings.TrimSpace(s.exec(ctx, router, s.K0sFullPath+" kubectl --kubeconfig=/tmp/admin.conf logs logger --tail=1")))
	}

	stopped := s.ControllerNode(2)
	s.T().Logf("Stopping %s", stopped)
	s.Require().NoError(s.StopController(stopped))
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, slices.Sorted(slices.Values(controllers[:2])), s.nextHops(ctx, family, vip, bits), "The stopped controller should withdraw its route")
	}, time.Minute, time.Second)

	s.T().Logf("Starting %s again", stopped)
	s.Require().NoError(s.StartController(stopped))
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		assert.Equal(c, slices.Sorted(slices.Values(controllers)), s.nextHops(ctx, family, vip, bits), "The restarted controller should advertise the VIP again")
	}, 3*time.Minute, time.Second)
}

func (s *cplbBGPSuite) exec(ctx context.Context, node, cmd string) string {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	output, err := ssh.ExecWithOutput(ctx, cmd)
	s.Require().NoError(err, "%s on %s failed: %s", cmd, node, output)
	return output
}

// Returns the next hops the router installed for the VIP, sorted.
func (s *cplbBGPSuite) nextHops(ctx context.Context, family, vip string, bits int) []string {
	var routes map[string][]struct {
		Nexthops []struct {
			IP     string `json:"ip"`
			Active bool   `json:"active"`
		} `json:"nexthops"`
		Installed bool `json:"installed"`
	}
	prefix := fmt.Sprintf("%s/%d", vip, bits)
	output := s.exec(ctx, s.WorkerNode(0), fmt.Sprintf("vtysh -c 'show %s route %s json'", strings.TrimSuffix(family, "v4"), prefix))
	s.Require().NoError(json.Unmarshal([]byte(output), &routes))

	var hops []string
	for _, route := range routes[prefix] {
		for _, hop := range route.Nexthops {
			if route.Installed && hop.Active && !slices.Contains(hops, hop.IP) {
				hops = append(hops, hop.IP)
			}
		}
	}
	slices.Sort(hops)
	return hops
}

// TestCPLBBGPSuite checks that the controllers advertise the control plane
// load balancing VIP over BGP, and that it works as the single external address.
func TestCPLBBGPSuite(t *testing.T) {
	s := &cplbBGPSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     2,
		},
	}
	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "ipv6") {
		s.isIPv6Only = true
		s.Networks = []string{"bridge-ipv6"}
		s.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle.tar"}
		s.K0sExtraImageBundleMountPoints = []string{"/var/lib/k0s/images/ipv6.tar"}
	}
	suite.Run(t, s)
}
