/*
Copyright 2021 k0s authors

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
package configcrd

import "github.com/k0sproject/k0s/inttest/common"

type CRDSuite struct {
	common.FootlooseSuite
}

const k0sCustomConfig = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  creationTimestamp: null
  name: k0s
spec:
  api:
    address: 10.0.56.91
    k0sApiPort: 9443
    port: 6443
    sans:
    - 10.0.56.91
    - 172.17.0.1
  controllerManager: {}
  images:
    calico:
      cni:
        image: quay.io/calico/cni
        version: v3.18.1
      kubecontrollers:
        image: quay.io/calico/kube-controllers
        version: v3.18.1
      node:
        image: quay.io/calico/node
        version: v3.18.1
    coredns:
      image: k8s.gcr.io/coredns/coredns
      version: v1.7.0
    default_pull_policy: IfNotPresent
    konnectivity:
      image: k8s.gcr.io/kas-network-proxy/proxy-agent
      version: v0.0.24
    kubeproxy:
      image: k8s.gcr.io/kube-proxy
      version: v1.22.2
    kuberouter:
      cni:
        image: docker.io/cloudnativelabs/kube-router
        version: v1.3.1
      cniInstaller:
        image: quay.io/k0sproject/cni-node
        version: 0.1.0
    metricsserver:
      image: k8s.gcr.io/metrics-server/metrics-server
      version: v0.5.0
  installConfig:
    users:
      etcdUser: etcd
      kineUser: kube-apiserver
      konnectivityUser: konnectivity-server
      kubeAPIserverUser: kube-apiserver
      kubeSchedulerUser: kube-scheduler
  konnectivity:
    adminPort: 8133
    agentPort: 8132
  network:
    calico: null
    dualStack: {}
    kubeProxy:
      mode: iptables
    kuberouter:
      autoMTU: true
      mtu: 0
      peerRouterASNs: ""
      peerRouterIPs: ""
    podCIDR: 10.244.0.0/16
    provider: kuberouter
    serviceCIDR: 10.96.0.0/12
  podSecurityPolicy:
    defaultPolicy: 00-k0s-privileged
`
