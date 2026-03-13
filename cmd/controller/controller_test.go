// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/stretchr/testify/assert"
)

func TestControllerCmd_Help(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Running controllers is only supported on Linux")
	}

	defaultConfigPath := strconv.Quote(constant.K0sConfigPathDefault)
	defaultDataDir := strconv.Quote(constant.DataDirDefault)

	var out strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"controller", "--help"})
	underTest.SetOut(&out)
	assert.NoError(t, underTest.Execute())

	assert.Equal(t, `Run controller

Usage:
  k0s controller [join-token] [flags]

Aliases:
  controller, server

Examples:
	Command to associate master nodes:
	CLI argument:
	$ k0s controller [join-token]

	or CLI flag:
	$ k0s controller --token-file [path_to_file]

	or environment variable:
	$ K0S_TOKEN=[token] k0s controller
	Note: Token can be passed either as a CLI argument, a flag, or an environment variable

Flags:
      --api-server-stop-timeout duration               time to wait for the API server to stop
  -c, --config string                                  config file, use '-' to read the config from stdin (default `+defaultConfigPath+`)
      --cri-socket string                              container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --data-dir string                                Data Directory for k0s. DO NOT CHANGE for an existing setup, things will break! (default `+defaultDataDir+`)
  -d, --debug                                          Debug logging (implies verbose logging)
      --debugListenOn string                           Http listenOn for Debug pprof handler (default ":6060")
      --disable-components strings                     disable components (valid items: applier-manager,autopilot,control-api,coredns,csr-approver,endpoint-reconciler,helm,konnectivity-server,kube-controller-manager,kube-proxy,kube-scheduler,metrics-server,network-provider,node-role,system-rbac,update-prober,windows-node,worker-config)
      --enable-cloud-provider                          Whether or not to enable cloud provider support in kubelet
      --enable-dynamic-config                          enable cluster-wide dynamic config based on custom resource
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --enable-metrics-scraper                         enable scraping metrics from the controller components (kube-scheduler, kube-controller-manager)
      --enable-worker                                  enable worker (default false)
      --feature-gates mapStringBool                    feature gates to enable (comma separated list of key=value pairs)
  -h, --help                                           help for controller
      --ignore-pre-flight-checks                       continue even if pre-flight checks fail
      --init-only                                      only initialize controller and exit
      --iptables-mode string                           iptables mode (valid values: nft, legacy, auto). default: auto
      --k0s-cloud-provider-port int                    the port that k0s-cloud-provider binds on (default 10258)
      --k0s-cloud-provider-update-frequency duration   the frequency of k0s-cloud-provider node updates (default 2m0s)
      --kube-controller-manager-extra-args string      extra args for kube-controller-manager
      --kubelet-extra-args string                      extra args for kubelet
      --kubelet-root-dir string                        Kubelet root directory for k0s
      --labels mapStringString                         Node labels, list of key=value pairs
  -l, --logging stringToString                         Logging Levels for the different components (default [containerd=info,etcd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1])
      --no-taints                                      disable default taints for controller node
      --profile string                                 worker profile to use on the node (default "default")
      --single                                         enable single node (implies --enable-worker, default false)
      --status-socket string                           Full file path to the socket file. (default: <rundir>/status.sock)
      --taints strings                                 Node taints, list of key=value:effect strings
      --token-file string                              Path to the file containing join-token.
  -v, --verbose                                        Verbose logging (default true)
`, out.String())
}

func TestControllerCmd_Flags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Running controllers is only supported on Linux")
	}

	t.Run("api-server-stop-timeout", func(t *testing.T) {
		expected := `invalid argument "0s" for "--api-server-stop-timeout" flag: must be positive`
		var stdout, stderr strings.Builder
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"controller", "--api-server-stop-timeout", "0s"})
		underTest.SetOut(&stdout)
		underTest.SetErr(&stderr)
		assert.ErrorContains(t, underTest.Execute(), expected)
		assert.Empty(t, stdout.String())
		assert.Equal(t, "Error: "+expected+"\n", stderr.String())
	})
}
