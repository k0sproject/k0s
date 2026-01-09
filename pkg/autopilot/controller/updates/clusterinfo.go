// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updates

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/build"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// ClusterInfo holds cluster related information that the update server can use to determine which updates to push to clusters
type ClusterInfo struct {
	K0sVersion             string
	StorageType            v1beta1.StorageType
	ClusterID              types.UID
	ControlPlaneNodesCount uint
	WorkerData             WorkerData
	CNIProvider            string
	Arch                   string
}

type WorkerData struct {
	Archs    map[string]int
	OSes     map[string]int
	Runtimes map[string]int
}

type ClusterInfoCollector struct {
	client          kubernetes.Interface
	storageType     v1beta1.StorageType
	networkProvider string
}

func (ci *ClusterInfo) AsMap() map[string]string {
	// Marshal and encode the worker data as a string
	wd, err := json.Marshal(ci.WorkerData)
	if err != nil {
		return map[string]string{}
	}
	workerData := base64.StdEncoding.EncodeToString(wd)
	return map[string]string{
		"K0S_StorageType":            string(ci.StorageType),
		"K0S_ClusterID":              "kube-system:" + string(ci.ClusterID),
		"K0S_ControlPlaneNodesCount": strconv.FormatUint(uint64(ci.ControlPlaneNodesCount), 10),
		"K0S_WorkerData":             workerData,
		"K0S_Version":                ci.K0sVersion,
		"K0S_CNIProvider":            ci.CNIProvider,
		"K0S_Arch":                   ci.Arch,
	}

}

func NewClusterInfoCollector(nodeConfig *v1beta1.ClusterConfig, client kubernetes.Interface) *ClusterInfoCollector {
	return &ClusterInfoCollector{
		client:          client,
		storageType:     nodeConfig.Spec.Storage.Type,
		networkProvider: nodeConfig.Spec.Network.Provider,
	}
}

// CollectData collects the cluster information
func (c *ClusterInfoCollector) CollectData(ctx context.Context) (*ClusterInfo, error) {
	ci := &ClusterInfo{
		K0sVersion:  build.Version,
		Arch:        runtime.GOARCH,
		CNIProvider: c.networkProvider,
		StorageType: c.storageType,
	}

	// Collect cluster ID
	ns, err := c.client.CoreV1().Namespaces().Get(ctx,
		metav1.NamespaceSystem,
		metav1.GetOptions{})
	if err != nil {
		return ci, fmt.Errorf("can't find kube-system namespace: %w", err)
	}

	ci.ClusterID = ns.UID

	// Collect worker node infos
	wns, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ci, err
	}

	ci.WorkerData = WorkerData{
		Archs:    make(map[string]int),
		OSes:     make(map[string]int),
		Runtimes: make(map[string]int),
	}
	for _, node := range wns.Items {
		arch := node.Status.NodeInfo.Architecture
		if _, ok := ci.WorkerData.Archs[arch]; !ok {
			ci.WorkerData.Archs[arch] = 0
		}
		ci.WorkerData.Archs[arch]++

		os := node.Status.NodeInfo.OSImage
		if _, ok := ci.WorkerData.OSes[os]; !ok {
			ci.WorkerData.OSes[os] = 0
		}
		ci.WorkerData.OSes[os]++

		runtime := node.Status.NodeInfo.ContainerRuntimeVersion
		if _, ok := ci.WorkerData.Runtimes[runtime]; !ok {
			ci.WorkerData.Runtimes[runtime] = 0
		}
		ci.WorkerData.Runtimes[runtime]++
	}

	// Collect control plane node count
	ci.ControlPlaneNodesCount, err = kubeutil.CountActiveControllerLeases(ctx, c.client)
	if err != nil {
		return ci, fmt.Errorf("can't collect control plane nodes count: %w", err)
	}

	return ci, nil
}
