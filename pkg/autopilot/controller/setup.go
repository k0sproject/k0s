//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/avast/retry-go"
	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// setup will go through all of the required setup operations that are required for autopilot.
func (c *rootController) setup(ctx context.Context) error {
	logger := c.log.WithField("component", "setup")

	controlNodeName, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname for signal node setup: %w", err)
	}

	kubeletNodeName := controlNodeName
	if c.enableWorker {
		kubeletNodeName = apcomm.FindKubeletHostname(c.cfg.KubeletExtraArgs)
	}

	logger.Infof("Using effective hostname = '%v', kubelet hostname = '%v'", controlNodeName, kubeletNodeName)

	if err := retry.Do(func() error {
		logger.Infof("Attempting to create controlnode '%s'", controlNodeName)
		if err := c.createControlNode(ctx, controlNodeName, kubeletNodeName); err != nil {
			return fmt.Errorf("create controlnode '%s' attempt failed, retrying: %w", controlNodeName, err)
		}

		return nil

	}); err != nil {
		return fmt.Errorf("failed to create controlnode '%s' after max attempts: %w", controlNodeName, err)
	}

	return nil
}

// createControlNode creates a new control node, ignoring errors if one already exists
// for this physical host.
func (sc *rootController) createControlNode(ctx context.Context, name, nodeName string) error {
	if !sc.apiAddress.IsValid() {
		return errors.New("no API address given")
	}
	apiAddress := sc.apiAddress.String()

	logger := sc.log.WithField("component", "setup")
	client, err := sc.kubeClientFactory.GetK0sClient()
	if err != nil {
		return err
	}

	// Create the ControlNode object if needed
	node, err := client.AutopilotV1beta2().ControlNodes().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logger.Info("Autopilot 'controlnodes' CRD not found, waiting...")
		if err := sc.waitForControlNodesCRD(ctx); err != nil {
			return fmt.Errorf("while waiting for autopilot 'controlnodes' CRD: %w", err)
		}

		logger.Info("Autopilot 'controlnodes' CRD found, continuing")

		logger.Infof("ControlNode '%s' not found, creating", name)
		mode := apconst.K0SControlNodeModeController
		if sc.enableWorker {
			mode = apconst.K0SControlNodeModeControllerWorker
		}
		node = &apv1beta2.ControlNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				// Create the usual os and arch labels as this describes a controller node
				Labels: map[string]string{
					corev1.LabelHostname:   nodeName,
					corev1.LabelOSStable:   runtime.GOOS,
					corev1.LabelArchStable: runtime.GOARCH,
				},
				Annotations: map[string]string{
					apconst.K0SControlNodeModeAnnotation: mode,
				},
			},
		}

		// Attempt to create the `controlnode`
		if node, err = client.AutopilotV1beta2().ControlNodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
			return err
		}
	} else if err != nil {
		logger.Errorf("unable to get controlnode '%s': %v", name, err)
		return err
	}

	node.Status = apv1beta2.ControlNodeStatus{
		Addresses: []corev1.NodeAddress{
			{Type: corev1.NodeInternalIP, Address: apiAddress},
			{Type: corev1.NodeHostName, Address: nodeName},
		},
		K0sVersion: build.Version,
	}

	logger.Infof("Updating controlnode status '%s'", name)
	if node, err = client.AutopilotV1beta2().ControlNodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
		logger.Errorf("unable to update controlnode '%s': %v", name, err)
		return err
	}
	logger.Infof("Updated controlnode '%s', status: %v", name, node.Status)

	return nil
}

// waitForControlNodesCRD waits until the controlnodes CRD is established for
// max 2 minutes.
func (sc *rootController) waitForControlNodesCRD(ctx context.Context) error {
	extClient, err := sc.kubeClientFactory.GetAPIExtensionsClient()
	if err != nil {
		return fmt.Errorf("unable to obtain extensions client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return watch.CRDs(extClient.ApiextensionsV1().CustomResourceDefinitions()).
		WithObjectName("controlnodes."+apv1beta2.GroupName).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryDelay, e := watch.IsRetryable(err); e == nil {
				sc.log.WithError(err).Debugf(
					"Encountered transient error while waiting for autopilot 'controlnodes' CRD, retrying in %s",
					retryDelay,
				)
				return retryDelay, nil
			}
			return 0, err
		}).
		Until(ctx, func(item *extensionsv1.CustomResourceDefinition) (bool, error) {
			for _, cond := range item.Status.Conditions {
				if cond.Type == extensionsv1.Established {
					return cond.Status == extensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})
}
