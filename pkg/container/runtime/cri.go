package runtime

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

var _ ContainerRuntime = &CRIRuntime{}

type CRIRuntime struct {
	criSocketPath string
}

func (cri *CRIRuntime) ListContainers() ([]string, error) {
	client, conn, err := getRuntimeClient(cri.criSocketPath)
	defer closeConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRI runtime client: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("failed to create CRI runtime client: %w", err)
	}
	request := &pb.ListPodSandboxRequest{}
	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(context.Background(), request)
	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, err
	}
	var pods []string
	for _, p := range r.GetItems() {
		pods = append(pods, p.Id)
	}
	return pods, nil
}

func (cri *CRIRuntime) RemoveContainer(id string) error {
	client, conn, err := getRuntimeClient(cri.criSocketPath)
	defer closeConnection(conn)
	if err != nil {
		return fmt.Errorf("failed to create CRI runtime client: %w", err)
	}
	if client == nil {
		return fmt.Errorf("failed to create CRI runtime client")
	}
	request := &pb.RemovePodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := client.RemovePodSandbox(context.Background(), request)
	logrus.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	logrus.Debugf("Removed pod sandbox %s\n", id)
	return nil
}

func (cri *CRIRuntime) StopContainer(id string) error {
	client, conn, err := getRuntimeClient(cri.criSocketPath)
	defer closeConnection(conn)
	if err != nil {
		return fmt.Errorf("failed to create CRI runtime client: %w", err)
	}
	if client == nil {
		return fmt.Errorf("failed to create CRI runtime client")
	}
	request := &pb.StopPodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(context.Background(), request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return fmt.Errorf("failed to stop pod sandbox: %w", err)
	}
	logrus.Debugf("Stopped pod sandbox %s\n", id)
	return nil
}

func getRuntimeClient(addr string) (pb.RuntimeServiceClient, *grpc.ClientConn, error) {
	conn, err := getRuntimeClientConnection(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	runtimeClient := pb.NewRuntimeServiceClient(conn)
	return runtimeClient, conn, nil
}

func getRuntimeClientConnection(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		errMsg := fmt.Errorf("connect endpoint %s, make sure you are running as root and the endpoint has been started: %w", addr, err)
		logrus.Error(errMsg)
	} else {
		logrus.Debugf("connected successfully using endpoint: %s", addr)
	}
	return conn, nil
}

func closeConnection(conn *grpc.ClientConn) {
	if conn == nil {
		return
	}
	conn.Close()
}
