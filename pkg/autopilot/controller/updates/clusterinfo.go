/*
Copyright 2023 k0s authors

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

package updates

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ClusterInfo holds cluster related information that the update server can use to determine which updates to push to clusters
type ClusterInfo struct {
	K0sVersion             string
	StorageType            string
	ClusterID              string
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

func (ci *ClusterInfo) AsMap() map[string]string {
	// Marshal and encode the worker data as a string
	wd, err := json.Marshal(ci.WorkerData)
	if err != nil {
		return map[string]string{}
	}
	workerData := base64.StdEncoding.EncodeToString(wd)
	return map[string]string{
		"K0S_StorageType":            ci.StorageType,
		"K0S_ClusterID":              ci.ClusterID,
		"K0S_ControlPlaneNodesCount": fmt.Sprintf("%d", ci.ControlPlaneNodesCount),
		"K0S_WorkerData":             workerData,
		"K0S_Version":                ci.K0sVersion,
		"K0S_CNIProvider":            ci.CNIProvider,
		"K0S_Arch":                   ci.Arch,
	}

}

// CollectData collects the cluster information
func CollectData(ctx context.Context, kc kubernetes.Interface) (*ClusterInfo, error) {
	ci := &ClusterInfo{}
	ci.K0sVersion = build.Version
	ci.Arch = runtime.GOARCH

	nodeConfig := k0scontext.FromContext[v1beta1.ClusterConfig](ctx, k0scontext.ContextNodeConfigKey)
	if nodeConfig != nil {
		ci.CNIProvider = nodeConfig.Spec.Network.Provider
		ci.StorageType = nodeConfig.Spec.Storage.Type
	}

	// Collect cluster ID
	ns, err := kc.CoreV1().Namespaces().Get(ctx,
		"kube-system",
		metav1.GetOptions{})
	if err != nil {
		return ci, fmt.Errorf("can't find kube-system namespace: %w", err)
	}

	ci.ClusterID = fmt.Sprintf("kube-system:%s", ns.UID)

	// Collect worker node infos
	wns, err := kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
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
	ci.ControlPlaneNodesCount, err = kubeutil.CountActiveControllerLeases(ctx, kc)
	if err != nil {
		return ci, fmt.Errorf("can't collect control plane nodes count: %w", err)
	}

	return ci, nil
}
