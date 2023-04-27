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

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-openapi/jsonpointer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// NodeRole implements the component interface to manage node role labels for worker nodes
type NodeRole struct {
	log logrus.FieldLogger

	kubeClientFactory k8sutil.ClientFactoryInterface
	k0sVars           *config.CfgVars
}

// NewNodeRole creates new NodeRole reconciler
func NewNodeRole(k0sVars *config.CfgVars, clientFactory k8sutil.ClientFactoryInterface) *NodeRole {
	return &NodeRole{
		log: logrus.WithFields(logrus.Fields{"component": "noderole"}),

		kubeClientFactory: clientFactory,
		k0sVars:           k0sVars,
	}
}

// Init no-op
func (n *NodeRole) Init(_ context.Context) error {
	return nil
}

// Run checks and adds labels
func (n *NodeRole) Start(ctx context.Context) error {
	client, err := n.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}
	go func() {
		timer := time.NewTicker(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				if err != nil {
					n.log.Errorf("failed to get node list: %v", err)
					continue
				}

				for _, node := range nodes.Items {
					err = n.ensureNodeLabel(ctx, client, node)
					if err != nil {
						n.log.Error(err)
					}
				}
			}
		}
	}()

	return nil
}

func (n *NodeRole) ensureNodeLabel(ctx context.Context, client kubernetes.Interface, node corev1.Node) error {
	var labelToAdd string
	for label, value := range node.Labels {
		if strings.HasPrefix(label, constant.NodeRoleLabelNamespace) {
			return nil
		}

		if label == constant.K0SNodeRoleLabel {
			labelToAdd = fmt.Sprintf("%s/%s", constant.NodeRoleLabelNamespace, value)
		}
	}

	if labelToAdd != "" {
		_, err := n.addNodeLabel(ctx, client, node.Name, labelToAdd, "true")
		if err != nil {
			return fmt.Errorf("failed to set label '%s' to node %s: %w", labelToAdd, node.Name, err)
		}
	}

	return nil
}

func (n *NodeRole) addNodeLabel(ctx context.Context, client kubernetes.Interface, node, key, value string) (*corev1.Node, error) {
	keyPath := fmt.Sprintf("/metadata/labels/%s", jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return client.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

// Stop no-op
func (n *NodeRole) Stop() error {
	return nil
}
