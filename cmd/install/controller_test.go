//go:build linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/stretchr/testify/assert"
)

func TestInstallCmd_Controller_Help(t *testing.T) {
	defaultConfigPath := strconv.Quote(constant.K0sConfigPathDefault)
	defaultDataDir := strconv.Quote(constant.DataDirDefault)

	var out strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"install", "controller", "--help"})
	underTest.SetOut(&out)
	assert.NoError(t, underTest.Execute())

	assert.Equal(t, `Install k0s controller on a brand-new system. Must be run as root (or with sudo)

Usage:
  k0s install controller [flags]

Aliases:
  controller, server

Examples:
All default values of controller command will be passed to the service stub unless overridden.

With the controller subcommand you can setup a single node cluster by running:

	k0s install controller --single
	

Flags:
  -c, --config string                                  config file, use '-' to read the config from stdin (default `+defaultConfigPath+`)
      --cri-socket string                              container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --data-dir string                                Data Directory for k0s. DO NOT CHANGE for an existing setup, things will break! (default `+defaultDataDir+`)
      --disable-components strings                     disable components (valid items: applier-manager,autopilot,control-api,coredns,csr-approver,endpoint-reconciler,helm,konnectivity-server,kube-controller-manager,kube-proxy,kube-scheduler,metrics-server,network-provider,node-role,system-rbac,update-prober,windows-node,worker-config)
      --enable-cloud-provider                          Whether or not to enable cloud provider support in kubelet
      --enable-dynamic-config                          enable cluster-wide dynamic config based on custom resource
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --enable-metrics-scraper                         enable scraping metrics from the controller components (kube-scheduler, kube-controller-manager)
      --enable-worker                                  enable worker (default false)
      --feature-gates mapStringBool                    feature gates to enable (comma separated list of key=value pairs)
  -h, --help                                           help for controller
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

Global Flags:
  -d, --debug                  Debug logging (implies verbose logging)
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -e, --env stringArray        Set environment variables (<name>=<value> or just <name>)
      --force                  Force init script creation
      --start                  Start the service immediately after installation
  -v, --verbose                Verbose logging
`, out.String())
}
