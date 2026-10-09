// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/sirupsen/logrus"
)

// K0sNodeLabeler labels each node with whether it runs k0s, while leading.
type K0sNodeLabeler struct {
	Clients       kubeutil.ClientFactoryInterface
	LeaderElector leaderelector.Interface

	log    logrus.FieldLogger
	stop   func()
	resync time.Duration
}

var _ manager.Component = (*K0sNodeLabeler)(nil)

func (l *K0sNodeLabeler) Init(context.Context) error {
	l.log = logrus.WithField("component", "k0s-node-labeler")
	if l.resync == 0 {
		l.resync = time.Minute
	}
	return nil
}

func (l *K0sNodeLabeler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var wg sync.WaitGroup
	wg.Go(func() {
		leaderelection.RunLeaderTasks(ctx, l.LeaderElector.CurrentStatus, l.labelNodes)
	})
	l.stop = func() { cancel(); wg.Wait() }
	return nil
}

func (l *K0sNodeLabeler) Stop() error {
	if l.stop != nil {
		l.stop()
	}
	return nil
}

// labelNodes labels the nodes until the context is done.
// The informer's resync retries failed patches.
func (l *K0sNodeLabeler) labelNodes(ctx context.Context) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		client, err := l.Clients.GetClient()
		if err != nil {
			l.log.WithError(err).Error("Failed to create a client, retrying")
			return
		}

		factory := informers.NewSharedInformerFactory(client, l.resync)
		defer factory.Shutdown()
		informer := factory.Core().V1().Nodes().Informer()
		handle := func(obj any) { l.labelNode(ctx, client, obj) }
		_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    handle,
			UpdateFunc: func(_, obj any) { handle(obj) },
		})
		if err != nil {
			l.log.WithError(err).Error("Failed to watch nodes, retrying")
			return
		}

		factory.Start(ctx.Done())
		<-ctx.Done()
	}, 10*time.Second)
}

func (l *K0sNodeLabeler) labelNode(ctx context.Context, client kubernetes.Interface, obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return
	}
	patch, err := k0sNodeLabelPatch(node)
	if err != nil || patch == nil {
		return
	}
	_, err = client.CoreV1().Nodes().Patch(ctx, node.Name, apitypes.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		l.log.WithError(err).Warnf("Failed to label node %s", node.Name)
		return
	}
	l.log.Infof("Labeled node %s", node.Name)
}

// k0sNodeLabelPatch returns the patch that sets the k0s label of the node, if needed.
// Workers set it to true themselves, and k0s builds its kubelet with a version suffix.
func k0sNodeLabelPatch(node *corev1.Node) ([]byte, error) {
	version := node.Status.NodeInfo.KubeletVersion
	if version == "" {
		// The kubelet didn't report its version yet.
		return nil, nil
	}
	value := node.Labels[constant.K0sNodeLabel]
	_, controller := node.Labels[constant.K0SNodeRoleLabel]
	want := "false"
	if value == "true" || controller || strings.HasSuffix(version, constant.K0sKubeletVersionSuffix) {
		want = "true"
	}
	if value == want {
		return nil, nil
	}
	return json.Marshal(map[string]any{"metadata": map[string]any{"labels": map[string]string{constant.K0sNodeLabel: want}}})
}
