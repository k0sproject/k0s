// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/go-openapi/jsonpointer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

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
	nodeRoleNamespace, _, _ := strings.Cut(constants.LabelNodeRoleControlPlane, "/")
	for label, value := range node.Labels {
		if labelNamespace, _, _ := strings.Cut(label, "/"); labelNamespace == nodeRoleNamespace {
			return nil
		}

		if label == constant.K0SNodeRoleLabel {
			labelToAdd = path.Join(nodeRoleNamespace, value)
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
	keyPath := path.Join("/metadata/labels", jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return client.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

// Stop no-op
func (n *NodeRole) Stop() error {
	return nil
}
