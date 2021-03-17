package crictl

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

type CriCtl struct {
	addr string
}

func NewCriCtl(addr string) *CriCtl {
	return &CriCtl{addr}
}

func (c *CriCtl) RemovePod(id string) error {
	client, conn, err := getRuntimeClient(c.addr)
	defer closeConnection(conn)
	if client == nil {
		return errors.Errorf("failed to create CRI runtime client")
	}
	if err != nil {
		return errors.Wrapf(err, "failed to create CRI runtime client")
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

func (c *CriCtl) StopPod(id string) error {
	client, conn, err := getRuntimeClient(c.addr)
	defer closeConnection(conn)
	if client == nil {
		return errors.Errorf("failed to create CRI runtime client")
	}
	if err != nil {
		return errors.Wrapf(err, "failed to create CRI runtime client")
	}
	request := &pb.StopPodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(context.Background(), request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return errors.Wrapf(err, "failed to stop pod sandbox")
	}
	logrus.Debugf("Stopped pod sandbox %s\n", id)
	return nil
}

func getRuntimeClient(addr string) (pb.RuntimeServiceClient, *grpc.ClientConn, error) {
	conn, err := getRuntimeClientConnection(addr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	runtimeClient := pb.NewRuntimeServiceClient(conn)
	return runtimeClient, conn, nil
}

func getRuntimeClientConnection(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		errMsg := errors.Wrapf(err, "connect endpoint %s, make sure you are running as root and the endpoint has been started", addr)
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
