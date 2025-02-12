// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var _ ContainerRuntime = (*CRIRuntime)(nil)

type CRIRuntime struct {
	grpcTarget string
}

func (cri *CRIRuntime) Ping(ctx context.Context) error {
	client, conn, err := newRuntimeClient(cri.grpcTarget)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = client.Version(ctx, &pb.VersionRequest{})
	return err
}

func (cri *CRIRuntime) ListContainers(ctx context.Context) ([]string, error) {
	client, conn, err := newRuntimeClient(cri.grpcTarget)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	request := &pb.ListPodSandboxRequest{}
	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(ctx, request)
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

func (cri *CRIRuntime) RemoveContainer(ctx context.Context, id string) error {
	client, conn, err := newRuntimeClient(cri.grpcTarget)
	if err != nil {
		return err
	}
	defer conn.Close()

	request := &pb.RemovePodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := client.RemovePodSandbox(ctx, request)
	logrus.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	logrus.Debugf("Removed pod sandbox %s\n", id)
	return nil
}

func (cri *CRIRuntime) StopContainer(ctx context.Context, id string) error {
	client, conn, err := newRuntimeClient(cri.grpcTarget)
	if err != nil {
		return err
	}
	defer conn.Close()

	request := &pb.StopPodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(ctx, request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return fmt.Errorf("failed to stop pod sandbox: %w", err)
	}
	logrus.Debugf("Stopped pod sandbox %s\n", id)
	return nil
}

func newRuntimeClient(target string) (pb.RuntimeServiceClient, io.Closer, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new gRPC client to target %s: %w", target, err)
	}

	return pb.NewRuntimeServiceClient(conn), conn, nil
}
