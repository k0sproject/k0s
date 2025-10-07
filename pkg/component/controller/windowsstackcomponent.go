// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"time"

	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/metadata"
	"k8s.io/utils/ptr"

	"github.com/sirupsen/logrus"
)

const WindowsStackName = "windows"

// WindowsStackComponent implements the component interface
// controller unpacks windows manifests
// if windows nodes are present in the cluster
type WindowsStackComponent struct {
	log                    logrus.FieldLogger
	client                 metadata.Interface
	updateWindowsNodeCount func(*uint)

	stop func()
}

// NewWindowsStackComponent creates new WindowsStackComponent reconciler
func NewWindowsStackComponent(clientFactory k8sutil.ClientFactoryInterface, updateWindowsNodeCount func(*uint)) (*WindowsStackComponent, error) {
	restConfig, err := clientFactory.GetRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := metadata.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &WindowsStackComponent{
		log:                    logrus.WithFields(logrus.Fields{"component": "WindowsNodeController"}),
		client:                 client,
		updateWindowsNodeCount: updateWindowsNodeCount,
	}, nil
}

// Init implements [manager.Component].
func (n *WindowsStackComponent) Init(context.Context) error {
	return nil
}

// Start implements [manager.Component].
func (n *WindowsStackComponent) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		var windowsNodeCount *uint
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if nodes, err := n.client.Resource(corev1.SchemeGroupVersion.WithResource("nodes")).List(ctx, metav1.ListOptions{
				LabelSelector: fields.OneTermEqualSelector(corev1.LabelOSStable, string(corev1.Windows)).String(),
			}); err != nil {
				if ctx.Err() == nil {
					n.log.WithError(err).Error("Failed to list Windows nodes")
				}
			} else if updateIfChanged(&windowsNodeCount, ptr.To(uint(len(nodes.Items)))) {
				n.log.WithField("count", *windowsNodeCount).Info("Windows node count changed")
				n.updateWindowsNodeCount(windowsNodeCount)
			} else {
				n.log.WithField("count", *windowsNodeCount).Debug("Windows node count unchanged")
			}
		}, 1*time.Minute)
	}()

	n.stop = func() { cancel(); <-done }
	return nil
}

// Stop implements [manager.Component].
func (n *WindowsStackComponent) Stop() error {
	if stop := n.stop; stop != nil {
		stop()
	}
	return nil
}
