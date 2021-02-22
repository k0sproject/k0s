package telemetry

import (
	"context"
	"fmt"

	analytics "github.com/segmentio/analytics-go"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/etcd"
)

type telemetryData struct {
	StorageType            string
	ClusterID              string
	Version                string
	WorkerNodesCount       int
	ControlPlaneNodesCount int
	WorkerData             []workerData
}

// Cannot use properly typed structs as they fail to be parsed properly on segment side :(
type workerData map[string]interface{}

func (td telemetryData) asProperties() analytics.Properties {
	return analytics.Properties{
		"storageType":            td.StorageType,
		"clusterID":              td.ClusterID,
		"workerNodesCount":       td.WorkerNodesCount,
		"controlPlaneNodesCount": td.ControlPlaneNodesCount,
		"version":                td.Version,
		"workerData":             td.WorkerData,
	}
}

func (c Component) collectTelemetry() (telemetryData, error) {
	var err error
	data := telemetryData{}

	data.Version = c.Version
	data.StorageType = c.getStorageType()
	data.ClusterID, err = c.getClusterID()

	if err != nil {
		return data, fmt.Errorf("can't collect cluster ID: %v", err)
	}
	wds, err := c.getWorkerData()
	if err != nil {
		return data, fmt.Errorf("can't collect workers count: %v", err)
	}

	data.WorkerNodesCount = len(wds)
	data.WorkerData = wds
	data.ControlPlaneNodesCount, err = c.getControlPlaneNodeCount()
	if err != nil {
		return data, fmt.Errorf("can't collect control plane nodes count: %v", err)
	}
	return data, nil
}

func (c Component) getStorageType() string {
	switch c.ClusterConfig.Spec.Storage.Type {
	case config.EtcdStorageType, config.KineStorageType:
		return c.ClusterConfig.Spec.Storage.Type
	}
	return "unknown"
}

func (c Component) getClusterID() (string, error) {
	ns, err := c.kubernetesClient.CoreV1().Namespaces().Get(context.Background(),
		"kube-system",
		metav1.GetOptions{})

	if err != nil {
		return "", fmt.Errorf("can't find kube-system namespace: %v", err)
	}

	return fmt.Sprintf("kube-system:%s", ns.UID), nil
}

func (c Component) getWorkerData() ([]workerData, error) {
	nodes, err := c.kubernetesClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	wds := make([]workerData, len(nodes.Items))

	for idx, n := range nodes.Items {
		wd := workerData{
			"os":   n.Status.NodeInfo.OSImage,
			"arch": n.Status.NodeInfo.Architecture,
			"cpus": n.Status.Capacity.Cpu().Value(),
			"mem":  n.Status.Capacity.Memory().ScaledValue(resource.Mega),
		}
		wds[idx] = wd
	}

	return wds, nil
}

func (c Component) getControlPlaneNodeCount() (int, error) {
	switch c.ClusterConfig.Spec.Storage.Type {
	case config.EtcdStorageType:
		cl, err := etcd.NewClient(c.K0sVars.CertRootDir, c.K0sVars.EtcdCertDir)
		if err != nil {
			return 0, fmt.Errorf("can't get etcd client: %v", err)
		}
		data, err := cl.ListMembers(context.Background())
		if err != nil {
			return 0, fmt.Errorf("can't receive etcd cluster members: %v", err)
		}
		return len(data), nil
	default:
		c.log.WithField("storageType", c.ClusterConfig.Spec.Storage.Type).Warning("can't get control planes count, unknown storage type")
		return -1, nil
	}
}

func (c Component) sendTelemetry() {
	data, err := c.collectTelemetry()
	if err != nil {
		c.log.WithError(err).Warning("can't prepare telemetry data")
		return
	}
	c.log.WithField("data", data).Info("sending telemetry")
	if err := c.analyticsClient.Enqueue(analytics.Track{
		AnonymousId: machineID(),
		Event:       heartbeatEvent,
		Properties:  data.asProperties(),
		Context: &analytics.Context{
			Direct: true,
		},
	}); err != nil {
		c.log.WithError(err).Warning("can't send telemetry data")
	}
}

func machineID() string {
	id, _ := util.MachineID()
	return id
}
