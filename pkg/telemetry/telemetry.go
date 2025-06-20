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

	"github.com/segmentio/analytics-go"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

type telemetryData struct {
	StorageType            string
	ClusterID              string
	WorkerNodesCount       int
	ControlPlaneNodesCount uint
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
		"storageType":            td.StorageType,
		"clusterID":              td.ClusterID,
		"workerNodesCount":       td.WorkerNodesCount,
		"controlPlaneNodesCount": int(td.ControlPlaneNodesCount),
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
		return data, fmt.Errorf("can't collect cluster ID: %w", err)
	}
	wds, sums, err := c.getWorkerData(ctx)
	if err != nil {
		return data, fmt.Errorf("can't collect workers count: %w", err)
	}

	data.WorkerNodesCount = len(wds)
	data.WorkerData = wds
	data.MEMTotal = sums.memTotal
	data.CPUTotal = sums.cpuTotal
	data.ControlPlaneNodesCount, err = kubeutil.CountActiveControllerLeases(ctx, c.kubernetesClient)
	if err != nil {
		return data, fmt.Errorf("can't collect control plane nodes count: %w", err)
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
		return "", fmt.Errorf("can't find kube-system namespace: %w", err)
	}

	return fmt.Sprintf("kube-system:%s", ns.UID), nil
}

func (c Component) getWorkerData(ctx context.Context) ([]workerData, workerSums, error) {
	nodes, err := c.kubernetesClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
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
	c.addCustomData(ctx, &hostData)

	c.log.WithField("data", data).WithField("hostdata", hostData).Info("sending telemetry")
	if err := c.analyticsClient.Enqueue(analytics.Track{
		AnonymousId: "(removed)",
		Event:       heartbeatEvent,
		Properties:  data.asProperties(),
		Context:     &hostData,
	}); err != nil {
		c.log.WithError(err).Warning("can't send telemetry data")
	}
}

func (c Component) addCustomData(ctx context.Context, analyticCtx *analytics.Context) {
	cm, err := c.kubernetesClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "k0s-telemetry", metav1.GetOptions{})
	if err != nil {
		return
	}
	for k, v := range cm.Data {
		analyticCtx.Extra[fmt.Sprintf("custom.%s", k)] = v
	}
}
