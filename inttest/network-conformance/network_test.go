// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
	sc "github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

type networkSuite struct {
	common.BootlooseSuite
	cni       string
	proxyMode string
}

func (s *networkSuite) TestK0sGetsUp() {
	// Build k0s config from structs and marshal to YAML
	{
		clusterCfg := &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: func() *v1beta1.Network {
					network := v1beta1.DefaultNetwork()
					network.Provider = s.cni
					if network.KubeProxy == nil {
						network.KubeProxy = v1beta1.DefaultKubeProxy()
					}
					network.KubeProxy.Mode = s.proxyMode
					return network
				}(),
			},
		}
		config, err := yaml.Marshal(clusterCfg)
		s.Require().NoError(err)
		s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	}

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)

	k8sVersion, err := kc.ServerVersion()
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	var daemonSetName string
	switch s.cni {
	case "calico":
		daemonSetName = "calico-node"
	case "kuberouter":
		daemonSetName = "kube-router"
	}
	s.T().Log("waiting to see CNI pods ready for", daemonSetName)
	s.NoErrorf(common.WaitForDaemonSet(s.Context(), kc, daemonSetName, "kube-system"), "%s did not start", daemonSetName)

	restConfig, err := s.GetKubeConfig("controller0")
	s.Require().NoError(err)

	skc, err := dynamic.NewAPIHelperFromRESTConfig(restConfig)
	s.Require().NoError(err)
	client, err := sc.NewSonobuoyClient(restConfig, skc)
	s.Require().NoError(err)

	deadline, _ := s.Context().Deadline()
	err = client.Run(&sc.RunConfig{
		GenConfig: sc.GenConfig{
			EnableRBAC:     true,
			DynamicPlugins: []string{"e2e"},
			PluginEnvOverrides: map[string]map[string]string{
				"e2e": {
					"E2E_FOCUS": "\\[sig-network\\].*\\[Conformance\\]",
					//
					// Skipping flaky tests:
					// - [It] [sig-network] Services should be able to switch session affinity for service with type clusterIP [LinuxOnly] [Conformance]
					// - [It] [sig-network] Services should have session affinity work for service with type clusterIP [LinuxOnly] [Conformance]
					// - [It] [sig-network] Services should have session affinity timeout work for service with type clusterIP [LinuxOnly] [Conformance]
					//
					"E2E_SKIP":          "\\[Serial\\]|(Services\\ should.*session\\ affinity\\ .*for\\ service\\ with\\ type\\ clusterIP)",
					"E2E_PARALLEL":      "y",
					"E2E_USE_GO_RUNNER": "true",
				},
			},
			KubeVersion: semver.Canonical(k8sVersion.String()),
		},
		Wait:       time.Until(deadline),
		WaitOutput: "Silent",
	})
	s.Require().NoError(err)
	status, err := client.GetStatus(&sc.StatusConfig{Namespace: "sonobuoy"})
	s.Require().NoError(err)

	s.T().Log("retrieving results")
	r, ec, err := client.RetrieveResults(&sc.RetrieveConfig{
		Namespace: "sonobuoy",
		Path:      "/tmp/sonobuoy",
	})
	s.Require().NoError(err)
	s.Require().NoError(retrieveResults(r, ec))

	s.T().Log("sonobuoy test status: ", status)
	s.Require().Equal("complete", status.Status)
	s.Require().Len(status.Plugins, 1)
	s.Require().Equal("passed", status.Plugins[0].ResultStatus)
}

func retrieveResults(r io.Reader, ec <-chan error) error {
	eg := &errgroup.Group{}
	eg.Go(func() error { return <-ec })
	eg.Go(func() error {
		filesCreated, err := sc.UntarAll(r, os.TempDir(), "")
		if err != nil {
			return err
		}
		for _, name := range filesCreated {
			fmt.Println(name)
		}
		return nil
	})

	return eg.Wait()
}

func TestNetworkSuite(t *testing.T) {
	s := networkSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		"kuberouter",
		"iptables",
	}

	target := os.Getenv("K0S_INTTEST_TARGET")
	if strings.Contains(target, "calico") {
		s.cni = "calico"
	}
	if strings.HasSuffix(target, "-nft") {
		s.proxyMode = "nftables"
	}

	t.Logf("Testing %s using %s", s.cni, s.proxyMode)
	suite.Run(t, &s)
}
