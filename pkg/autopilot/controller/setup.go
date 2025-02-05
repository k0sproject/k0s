//go:build unix

// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"fmt"
	"runtime"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetupController defines operations that should be run once to completion,
// typically at autopilot startup.
type SetupController interface {
	Run(ctx context.Context) error
}

type setupController struct {
	log              *logrus.Entry
	clientFactory    apcli.FactoryInterface
	k0sDataDir       string
	enableWorker     bool
	kubeletExtraArgs string
}

var _ SetupController = (*setupController)(nil)

// NewSetupController creates a `SetupController`
func NewSetupController(logger *logrus.Entry, cf apcli.FactoryInterface, k0sDataDir, kubeletExtraArgs string, enableWorker bool) SetupController {
	return &setupController{
		log:              logger.WithField("controller", "setup"),
		clientFactory:    cf,
		k0sDataDir:       k0sDataDir,
		kubeletExtraArgs: kubeletExtraArgs,
		enableWorker:     enableWorker,
	}
}

// Run will go through all of the required setup operations that are required for autopilot.
// This effectively replaces the manifest concept used in k0s, as there is no guarantee that
// autopilot has access to the k0s file-system, or even if k0s is used at all.
func (sc *setupController) Run(ctx context.Context) error {
	logger := sc.log.WithField("component", "setup")

	logger.Infof("Creating namespace '%s'", apconst.AutopilotNamespace)
	if _, err := createNamespace(ctx, sc.clientFactory, apconst.AutopilotNamespace); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create required namespace '%s'", apconst.AutopilotNamespace)
		}
	}

	controlNodeName, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname for signal node setup: %w", err)
	}

	kubeletNodeName := controlNodeName
	if sc.enableWorker {
		kubeletNodeName = apcomm.FindKubeletHostname(sc.kubeletExtraArgs)
	}

	logger.Infof("Using effective hostname = '%v', kubelet hostname = '%v'", controlNodeName, kubeletNodeName)

	if err := retry.Do(func() error {
		logger.Infof("Attempting to create controlnode '%s'", controlNodeName)
		if err := sc.createControlNode(ctx, sc.clientFactory, controlNodeName, kubeletNodeName); err != nil {
			return fmt.Errorf("create controlnode '%s' attempt failed, retrying: %w", controlNodeName, err)
		}

		return nil

	}); err != nil {
		return fmt.Errorf("failed to create controlnode '%s' after max attempts: %w", controlNodeName, err)
	}

	return nil
}

// createNamespace creates a namespace with the provided name
func createNamespace(ctx context.Context, cf apcli.FactoryInterface, name string) (*corev1.Namespace, error) {
	client, err := cf.GetClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create obtain a kube client: %w", err)
	}

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return client.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
}

// createControlNode creates a new control node, ignoring errors if one already exists
// for this physical host.
func (sc *setupController) createControlNode(ctx context.Context, cf apcli.FactoryInterface, name, nodeName string) error {
	logger := sc.log.WithField("component", "setup")
	client, err := sc.clientFactory.GetK0sClient()
	if err != nil {
		return err
	}

	// Create the ControlNode object if needed
	node, err := client.AutopilotV1beta2().ControlNodes().Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		logger.Info("Autopilot 'controlnodes' CRD not found, waiting...")
		if err := sc.waitForControlNodesCRD(ctx, cf); err != nil {
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

	addresses, err := getControlNodeAddresses(nodeName)
	if err != nil {
		return err
	}

	node.Status = apv1beta2.ControlNodeStatus{
		Addresses: addresses,
	}

	logger.Infof("Updating controlnode status '%s'", name)
	if node, err = client.AutopilotV1beta2().ControlNodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
		logger.Errorf("unable to update controlnode '%s': %v", name, err)
		return err
	}
	logger.Infof("Updated controlnode '%s', status: %v", name, node.Status)

	return nil
}

// TODO re-use from somewhere else
const DefaultK0sStatusSocketPath = "/run/k0s/status.sock"

func getControlNodeAddresses(hostname string) ([]corev1.NodeAddress, error) {
	addresses := []corev1.NodeAddress{}
	apiAddress, err := getControllerAPIAddress()
	if err != nil {
		return addresses, err
	}
	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: apiAddress,
	})

	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeHostName,
		Address: hostname,
	})

	return addresses, nil
}

func getControllerAPIAddress() (string, error) {
	status, err := status.GetStatusInfo(DefaultK0sStatusSocketPath)
	if err != nil {
		return "", err
	}

	return status.ClusterConfig.Spec.API.Address, nil
}

// waitForControlNodesCRD waits until the controlnodes CRD is established for
// max 2 minutes.
func (sc *setupController) waitForControlNodesCRD(ctx context.Context, cf apcli.FactoryInterface) error {
	extClient, err := cf.GetExtensionClient()
	if err != nil {
		return fmt.Errorf("unable to obtain extensions client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return watch.CRDs(extClient.CustomResourceDefinitions()).
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
