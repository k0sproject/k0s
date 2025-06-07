/*
Copyright 2025 k0s authors

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

package worker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/go-openapi/jsonpointer"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/sirupsen/logrus"
)

type NodeRole struct {
	KubeconfigGetter clientcmd.KubeconfigGetter
	NodeName         apitypes.NodeName

	stop func()
}

// Init implements [manager.Component].
func (n *NodeRole) Init(context.Context) error { return nil }

// Start implements [manager.Component].
func (n *NodeRole) Start(context.Context) error {
	log := logrus.WithFields(logrus.Fields{
		"component": "node-role",
		"node":      n.NodeName,
	})

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		for {
			client, err := kubernetes.NewClient(n.KubeconfigGetter)
			if err != nil {
				log.WithError(err).Error("Failed to create Kubernetes client")
				continue
			}

			nodes := client.CoreV1().Nodes()
			err = watch.Nodes(nodes).
				WithObjectName(string(n.NodeName)).
				WithErrorCallback(watch.IsRetryable).
				Until(ctx, func(node *corev1.Node) (bool, error) {
					if _, exists := node.Labels[constants.LabelNodeRoleControlPlane]; exists {
						log.Info("Control-plane label exists")
						return true, nil
					}

					patch := fmt.Sprintf(`[{"op":"add", "path":"/metadata/labels/%s", "value":"true"}]`, jsonpointer.Escape(constants.LabelNodeRoleControlPlane))
					if _, err := nodes.Patch(ctx, node.Name, apitypes.JSONPatchType, []byte(patch), metav1.PatchOptions{FieldManager: "k0s"}); err != nil {
						return false, err
					}

					log.Info("Control-plane label set")
					return true, nil
				})
			if err == nil {
				return // success, end the loop
			}

			log.WithError(err).Error("Failed to set control-plane label")

			select {
			case <-time.After(20 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	n.stop = func() { cancel(); <-stopped }

	return nil
}

// Stop implements [manager.Component].
func (n *NodeRole) Stop() error {
	if n.stop != nil {
		n.stop()
	}

	return nil
}
