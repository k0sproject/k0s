package telemetry

import (
	"context"
	"fmt"
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/etcd"
	"github.com/Mirantis/mke/pkg/util"
	"gopkg.in/segmentio/analytics-go.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type telemetryData struct {
	StorageType            string
	ClusterID              string
	WorkerNodesCount       int
	ControlPlaneNodesCount int
}

func (td telemetryData) asProperties() analytics.Properties {
	return analytics.Properties{
		"storageType":            td.StorageType,
		"clusterID":              td.ClusterID,
		"workerNodesCount":       td.WorkerNodesCount,
		"controlPlaneNodesCount": td.ControlPlaneNodesCount,
	}
}

func (p Component) collectTelemetry() (telemetryData, error) {
	var err error
	data := telemetryData{}

	data.StorageType = p.getStorageType()
	data.ClusterID, err = p.getClusterID()

	if err != nil {
		return data, fmt.Errorf("can't collect cluster ID: %v", err)
	}
	data.WorkerNodesCount, err = p.getWorkerNodeCount()
	if err != nil {
		return data, fmt.Errorf("can't collect workers count: %v", err)
	}
	data.ControlPlaneNodesCount, err = p.getControlPlaneNodeCount()
	if err != nil {
		return data, fmt.Errorf("can't collect control plane nodes count: %v", err)
	}
	return data, nil
}

func (p Component) getStorageType() string {
	switch p.ClusterConfig.Spec.Storage.Type {
	case config.EtcdStorageType, config.KineStorageType:
		return p.ClusterConfig.Spec.Storage.Type
	}
	return "unknown"
}

func (p Component) getClusterID() (string, error) {
	nss, err := p.kubernetesClient.CoreV1().Namespaces().List(context.Background(),
		metav1.ListOptions{})

	if err != nil {
		return "", err
	}

	for _, ns := range nss.Items {
		if ns.Name != "kube-system" {
			continue
		}
		return fmt.Sprintf("kube-system:%s", ns.UID), nil
	}

	return "", fmt.Errorf("can't find kube-system namespace")
}

func (p Component) getWorkerNodeCount() (int, error) {
	nodes, err := p.kubernetesClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(nodes.Items), nil
}

func (p Component) getControlPlaneNodeCount() (int, error) {
	switch p.ClusterConfig.Spec.Storage.Type {
	case config.EtcdStorageType:
		cl, err := etcd.NewClient()
		if err != nil {
			return 0, fmt.Errorf("can't get etcd client: %v", err)
		}
		data, err := cl.ListMembers(context.Background())
		if err != nil {
			return 0, fmt.Errorf("can't receive etcd cluster members: %v", err)
		}
		return len(data), nil
	default:
		p.log.WithField("storageType", p.ClusterConfig.Spec.Storage.Type).Warning("can't get control planes count, unknown storage type")
		return -1, nil
	}
}

func (p Component) sendTelemetry() {
	data, err := p.collectTelemetry()
	if err != nil {
		p.log.WithError(err).Warning("can't prepare telemetry data")
		return
	}
	p.log.WithField("data", data).Info("sending telemetry")
	if err := p.analyticsClient.Enqueue(analytics.Track{
		AnonymousId: machineID(),
		Event:       heartbeatEvent,
		Properties:  data.asProperties(),
	}); err != nil {
		p.log.WithError(err).Warning("can't send telemetry data")
	}
}

func machineID() string {
	id, _ := util.MachineID()
	return id
}
