/*
Copyright 2020 k0s authors

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

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/build"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/segmentio/analytics-go"
)

type telemetryData struct {
	StorageType            v1beta1.StorageType
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
	cpuTotal int64
	memTotal int64
}

func (td telemetryData) asProperties() analytics.Properties {
	return analytics.Properties{
		"storageType":            string(td.StorageType),
		"clusterID":              td.ClusterID,
		"workerNodesCount":       td.WorkerNodesCount,
		"controlPlaneNodesCount": td.ControlPlaneNodesCount,
		"workerData":             td.WorkerData,
		"memTotal":               td.MEMTotal,
		"cpuTotal":               td.CPUTotal,
	}
}

func (c *Component) collectTelemetry(ctx context.Context, clients kubernetes.Interface) (telemetryData, error) {
	var err error
	data := telemetryData{}

	data.StorageType = c.StorageType
	data.ClusterID, err = getClusterID(ctx, clients)

	if err != nil {
		return data, fmt.Errorf("can't collect cluster ID: %w", err)
	}
	wds, sums, err := getWorkerData(ctx, clients)
	if err != nil {
		return data, fmt.Errorf("can't collect workers count: %w", err)
	}

	data.WorkerNodesCount = len(wds)
	data.WorkerData = wds
	data.MEMTotal = sums.memTotal
	data.CPUTotal = sums.cpuTotal
	data.ControlPlaneNodesCount, err = kubeutil.GetControlPlaneNodeCount(ctx, clients)
	if err != nil {
		return data, fmt.Errorf("can't collect control plane nodes count: %w", err)
	}
	return data, nil
}

func getClusterID(ctx context.Context, clients kubernetes.Interface) (string, error) {
	ns, err := clients.CoreV1().Namespaces().Get(ctx,
		"kube-system",
		metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("can't find kube-system namespace: %w", err)
	}

	return fmt.Sprintf("kube-system:%s", ns.UID), nil
}

func getWorkerData(ctx context.Context, clients kubernetes.Interface) ([]workerData, workerSums, error) {
	nodes, err := clients.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, workerSums{}, err
	}

	wds := make([]workerData, len(nodes.Items))
	var memTotal int64
	var cpuTotal int64
	for idx, n := range nodes.Items {
		wd := workerData{
			"os":      n.Status.NodeInfo.OSImage,
			"arch":    n.Status.NodeInfo.Architecture,
			"cpus":    n.Status.Capacity.Cpu().Value(),
			"mem":     n.Status.Capacity.Memory().ScaledValue(resource.Mega),
			"runtime": n.Status.NodeInfo.ContainerRuntimeVersion,
		}
		wds[idx] = wd
		memTotal += n.Status.Capacity.Memory().ScaledValue(resource.Mega)
		cpuTotal += n.Status.Capacity.Cpu().Value()
	}

	return wds, workerSums{cpuTotal: cpuTotal, memTotal: memTotal}, nil
}

func (c *Component) sendTelemetry(ctx context.Context, analyticsClient analytics.Client, clients kubernetes.Interface) {
	data, err := c.collectTelemetry(ctx, clients)
	if err != nil {
		c.log.WithError(err).Warning("can't prepare telemetry data")
		return
	}

	hostData := analytics.Context{
		Extra: map[string]interface{}{"direct": true},
	}

	hostData.App.Version = build.Version
	hostData.App.Name = "k0s"
	hostData.App.Namespace = "k0s"
	hostData.Extra["cpuArch"] = runtime.GOARCH

	addSysInfo(&hostData)
	addCustomData(ctx, &hostData, clients)

	c.log.WithField("data", data).WithField("hostdata", hostData).Info("sending telemetry")
	if err := analyticsClient.Enqueue(analytics.Track{
		AnonymousId: "(removed)",
		Event:       heartbeatEvent,
		Properties:  data.asProperties(),
		Context:     &hostData,
	}); err != nil {
		c.log.WithError(err).Warning("can't send telemetry data")
	}
}

func addCustomData(ctx context.Context, analyticCtx *analytics.Context, clients kubernetes.Interface) {
	cm, err := clients.CoreV1().ConfigMaps("kube-system").Get(ctx, "k0s-telemetry", metav1.GetOptions{})
	if err != nil {
		return
	}
	for k, v := range cm.Data {
		analyticCtx.Extra[fmt.Sprintf("custom.%s", k)] = v
	}
}
