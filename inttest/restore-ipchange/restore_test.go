// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package basic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	testifysuite "github.com/stretchr/testify/suite"
)

type suite struct {
	common.BootlooseSuite
}

func (s *suite) TestRestoreIPChange() {
	ctx := s.TContext()

	{ // Pin the etcd peer address on all nodes.
		for i := range s.ControllerCount {
			config, err := yaml.Marshal(&k0sv1beta1.ClusterConfig{
				Spec: &k0sv1beta1.ClusterSpec{
					Storage: &k0sv1beta1.StorageSpec{
						Etcd: &k0sv1beta1.EtcdConfig{
							PeerAddress: s.GetIPAddress(s.ControllerNode(i)),
						},
					},
					Network: &k0sv1beta1.Network{
						NodeLocalLoadBalancing: &k0sv1beta1.NodeLocalLoadBalancing{
							Enabled: true,
						},
					},
				},
			})
			s.Require().NoError(err)
			s.WriteFileContent(s.ControllerNode(i), "/tmp/k0s.yaml", config)
		}
	}

	s.T().Log("Start first controller and wait for the cluster to become ready")
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker"))
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kc))
	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")
	s.Require().NoError(common.WaitForDaemonSet(ctx, kc, "konnectivity-agent", metav1.NamespaceSystem))

	s.T().Log("Make a snapshot, take a backup and wipe the first controller")
	snapshot, err := makeSnapshot(ctx, kc)
	s.Require().NoError(err)
	backup, err := s.takeBackup(ctx, s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	s.reset(ctx, s.ControllerNode(0))

	s.T().Log("Restore the backup on the second controller and launch it")
	s.Require().NoError(s.restoreBackup(ctx, s.ControllerNode(1), backup))
	s.Require().NoError(s.InitController(1, "--config=/tmp/k0s.yaml", "--enable-worker"))

	s.T().Log("Get a join token from the second controller")
	kc, err = s.KubeClient(s.ControllerNode(1))
	s.Require().NoError(err)

	s.T().Log("Join the other controllers in the usual way")
	token, err := s.JoinToken(ctx, s.ControllerNode(1), "controller")
	s.Require().NoError(err)

	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(1)))
	for _, i := range [...]int{2, 0} {
		s.Require().NoError(s.InitController(i, "--enable-worker", token))
	}
	for i := range s.ControllerCount {
		s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(i), kc))
	}

	s.T().Log("Wait until the cluster becomes ready")
	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")
	s.Require().NoError(common.WaitForDaemonSet(ctx, kc, "konnectivity-agent", metav1.NamespaceSystem))

	s.T().Log("Take another snapshot and compare it")
	snapshotAfterBackup, err := makeSnapshot(ctx, kc)
	s.Require().NoError(err)
	// Retain only the initial node
	for uid, name := range snapshotAfterBackup.nodes {
		if name != s.ControllerNode(0) {
			delete(snapshotAfterBackup.nodes, uid)
		}
	}
	s.Require().Equal(snapshot, snapshotAfterBackup)
}

func (s *suite) takeBackup(ctx context.Context, nodeName string) (_ []byte, err error) {
	ssh, err := s.SSH(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	var buf bytes.Buffer
	if err := ssh.Exec(ctx, "/usr/local/bin/k0s backup --debug --save-path -", common.SSHStreams{
		Out: &buf,
	}); err != nil {
		return nil, err
	}
	data := buf.Bytes()

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("bad backup: %w", err)
	}
	defer func() { err = errors.Join(err, gz.Close()) }()
	r := tar.NewReader(gz)

	for {
		if _, err := r.Next(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("bad backup: %w", err)
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, fmt.Errorf("bad backup: %w", err)
		}
	}

	return data, nil
}

func (s *suite) restoreBackup(ctx context.Context, nodeName string, backup []byte) error {
	ssh, err := s.SSH(ctx, nodeName)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	if err = ssh.Exec(ctx, "k0s restore --debug -", common.SSHStreams{
		In: bytes.NewReader(backup),
	}); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	return nil
}

func (s *suite) reset(ctx context.Context, name string) {
	ssh, err := s.SSH(ctx, name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(ctx, "k0s reset --debug", common.SSHStreams{}))
}

type snapshot struct {
	namespaces map[types.UID]string
	services   map[types.UID]string
	nodes      map[types.UID]string
}

func makeSnapshot(ctx context.Context, kc *kubernetes.Clientset) (s snapshot, err error) {
	// Take some UIDs to be able to verify state has restored properly
	if namespaces, err := kc.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); err != nil {
		return s, err
	} else {
		s.namespaces = make(map[types.UID]string, len(namespaces.Items))
		for _, n := range namespaces.Items {
			s.namespaces[n.UID] = n.Name
		}
	}

	if services, err := kc.CoreV1().Services(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{}); err != nil {
		return s, err
	} else {
		s.services = make(map[types.UID]string, len(services.Items))
		for _, svc := range services.Items {
			s.services[svc.UID] = svc.Name
		}
	}

	if nodes, err := kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		return s, err
	} else {
		s.nodes = make(map[types.UID]string)
		for _, n := range nodes.Items {
			s.nodes[n.UID] = n.Name
		}
	}

	return s, nil
}

func TestRestoreSuite(t *testing.T) {
	s := suite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     0,
		},
	}

	testifysuite.Run(t, &s)
}
