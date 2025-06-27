/*
Copyright 2021 k0s authors

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
	target      string
	dialOptions []grpc.DialOption
}

var defaultGRPCDialOptions = []grpc.DialOption{
	grpc.WithTransportCredentials(insecure.NewCredentials()),
}

func (cri *CRIRuntime) Ping(ctx context.Context) error {
	client, conn, err := cri.newRuntimeClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = client.Version(ctx, &pb.VersionRequest{})
	return err
}

func (cri *CRIRuntime) ListContainers(ctx context.Context) ([]string, error) {
	client, conn, err := cri.newRuntimeClient()
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
	client, conn, err := cri.newRuntimeClient()
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
	client, conn, err := cri.newRuntimeClient()
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

func (cri *CRIRuntime) newRuntimeClient() (pb.RuntimeServiceClient, io.Closer, error) {
	conn, err := grpc.NewClient(cri.target, cri.dialOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new gRPC client to target %s: %w", cri.target, err)
	}

	return pb.NewRuntimeServiceClient(conn), conn, nil
}
