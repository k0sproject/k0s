/*
Copyright 2022 k0s authors

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

package telemetry

import (
	"context"
	"fmt"
	"runtime"

	"github.com/segmentio/analytics-go"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/machineid"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

type telemetryData struct {
	StorageType            string
	ClusterID              string
	WorkerNodesCount       int
	ControlPlaneNodesCount int
	WorkerData             []workerData
	CPUTotal               int64
	MEMTotal               int64
}

// Cannot use properly typed structs as they fail to be parsed properly on segment side :(
type workerData map[string]interface{}

type workerSums struct {
	cpuTotal   int64
	memTotal   int64
	nodesTotal int
}

func (td telemetryData) asProperties() analytics.Properties {
	return analytics.Properties{
		"storageType":            td.StorageType,
		"clusterID":              td.ClusterID,
		"workerNodesCount":       td.WorkerNodesCount,
		"controlPlaneNodesCount": td.ControlPlaneNodesCount,
		"workerData":             td.WorkerData,
		"memTotal":               td.MEMTotal,
		"cpuTotal":               td.CPUTotal,
	}
}

func (c Component) collectTelemetry(ctx context.Context) (telemetryData, error) {
	var err error
	data := telemetryData{}

	data.StorageType = c.getStorageType()
	data.ClusterID, err = c.getClusterID(ctx)

	if err != nil {
		return data, fmt.Errorf("can't collect cluster ID: %v", err)
	}
	wds, sums, err := c.getWorkerData(ctx)
	if err != nil {
		return data, fmt.Errorf("can't collect workers count: %v", err)
	}

	data.WorkerNodesCount = sums.nodesTotal
	data.WorkerData = wds
	data.MEMTotal = sums.memTotal
	data.CPUTotal = sums.cpuTotal
	data.ControlPlaneNodesCount, err = kubeutil.GetControlPlaneNodeCount(ctx, c.kubernetesClient)
	if err != nil {
		return data, fmt.Errorf("can't collect control plane nodes count: %v", err)
	}
	return data, nil
}

func (c Component) getStorageType() string {
	switch c.clusterConfig.Spec.Storage.Type {
	case v1beta1.EtcdStorageType, v1beta1.KineStorageType:
		return c.clusterConfig.Spec.Storage.Type
	}
	return "unknown"
}

func (c Component) getClusterID(ctx context.Context) (string, error) {
	ns, err := c.kubernetesClient.CoreV1().Namespaces().Get(ctx,
		"kube-system",
		metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("can't find kube-system namespace: %v", err)
	}

	return fmt.Sprintf("kube-system:%s", ns.UID), nil
}

func (c Component) getWorkerGroup(os string, arch string, cpus int64, mem int64, runtime string) string {
	// go maps are hashing the string pretty fast anyway so I don't think we need
	// to do anything fancy here
	return fmt.Sprintf("%s:%s:%d:%d:%s", os, arch, cpus, mem, runtime)
}

func (c Component) getWorkerData(ctx context.Context) ([]workerData, workerSums, error) {
	nodes, err := c.kubernetesClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, workerSums{}, err
	}

	wm := make(map[string]workerData)
	var memTotal int64
	var cpuTotal int64
	for _, n := range nodes.Items {
		os := n.Status.NodeInfo.OSImage
		arch := n.Status.NodeInfo.Architecture
		cpus := n.Status.Capacity.Cpu().Value()
		mem := n.Status.Capacity.Memory().ScaledValue(resource.Mega)
		runtime := n.Status.NodeInfo.ContainerRuntimeVersion
		wg := c.getWorkerGroup(os, arch, cpus, mem, runtime)
		if wm[wg] == nil {
			wm[wg] = workerData{
				"os":      os,
				"arch":    arch,
				"cpus":    cpus,
				"mem":     mem,
				"runtime": runtime,
				"count":   0,
			}
		}
		wm[wg]["count"] = wm[wg]["count"].(int) + 1
		memTotal += mem
		cpuTotal += cpus
	}

	// convert map to slice so that we send it to segment without the key of the map
	wds := make([]workerData, len(wm))
	i := 0
	for _, wd := range wm {
		wds[i] = wd
		i++
	}

	return wds, workerSums{cpuTotal: cpuTotal, memTotal: memTotal, nodesTotal: len(nodes.Items)}, nil
}

func (c Component) sendTelemetry(ctx context.Context) {
	data, err := c.collectTelemetry(ctx)
	if err != nil {
		c.log.WithError(err).Warning("can't prepare telemetry data")
		return
	}

	hostData := analytics.Context{
		Extra: map[string]interface{}{"direct": true},
	}

	hostData.App.Version = c.Version
	hostData.App.Name = "k0s"
	hostData.App.Namespace = "k0s"
	hostData.Extra["cpuArch"] = runtime.GOARCH

	addSysInfo(&hostData)

	c.log.WithField("data", data).WithField("hostdata", hostData).Info("sending telemetry")
	if err := c.analyticsClient.Enqueue(analytics.Track{
		AnonymousId: machineID(),
		Event:       heartbeatEvent,
		Properties:  data.asProperties(),
		Context:     &hostData,
	}); err != nil {
		c.log.WithError(err).Warning("can't send telemetry data")
	}
}

func machineID() string {
	id, _ := machineid.Generate()
	return id.ID()
}
