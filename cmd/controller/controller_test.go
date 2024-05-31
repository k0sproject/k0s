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

package controller_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/stretchr/testify/assert"
)

func TestControllerCmd_Help(t *testing.T) {
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
	Note: Token can be passed either as a CLI argument or as a flag

Flags:
      --cidr-range string                              HACK: cidr range for the windows worker node (default "10.96.0.0/12")
  -c, --config string                                  config file, use '-' to read the config from stdin (default `+defaultConfigPath+`)
      --cri-socket string                              container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --data-dir string                                Data Directory for k0s. DO NOT CHANGE for an existing setup, things will break! (default `+defaultDataDir+`)
  -d, --debug                                          Debug logging (default: false)
      --debugListenOn string                           Http listenOn for Debug pprof handler (default ":6060")
      --disable-components strings                     disable components (valid items: applier-manager,autopilot,control-api,coredns,csr-approver,endpoint-reconciler,helm,konnectivity-server,kube-controller-manager,kube-proxy,kube-scheduler,metrics-server,network-provider,node-role,system-rbac,windows-node,worker-config)
      --enable-cloud-provider                          Whether or not to enable cloud provider support in kubelet
      --enable-dynamic-config                          enable cluster-wide dynamic config based on custom resource
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --enable-metrics-scraper                         enable scraping metrics from the controller components (kube-scheduler, kube-controller-manager)
      --enable-worker                                  enable worker (default false)
  -h, --help                                           help for controller
      --ignore-pre-flight-checks                       continue even if pre-flight checks fail
      --iptables-mode string                           iptables mode (valid values: nft, legacy, auto). default: auto
      --k0s-cloud-provider-port int                    the port that k0s-cloud-provider binds on (default 10258)
      --k0s-cloud-provider-update-frequency duration   the frequency of k0s-cloud-provider node updates (default 2m0s)
      --kube-controller-manager-extra-args string      extra args for kube-controller-manager
      --kubelet-extra-args string                      extra args for kubelet
      --labels strings                                 Node labels, list of key=value pairs
  -l, --logging stringToString                         Logging Levels for the different components (default [containerd=info,etcd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1])
      --no-taints                                      disable default taints for controller node
      --profile string                                 worker profile to use on the node (default "default")
      --single                                         enable single node (implies --enable-worker, default false)
      --status-socket string                           Full file path to the socket file. (default: <rundir>/status.sock)
      --taints strings                                 Node taints, list of key=value:effect strings
      --token-file string                              Path to the file containing join-token.
  -v, --verbose                                        Verbose logging (default: false)
`, out.String())
}
