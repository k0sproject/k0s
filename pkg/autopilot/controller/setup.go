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
	"os"
	"path"
	"runtime"
	"time"

	aptw "github.com/k0sproject/k0s/internal/autopilot/pkg/templatewriter"
	apv1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apem "github.com/k0sproject/k0s/static"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k0sinstall "github.com/k0sproject/k0s/pkg/install"
)

const (
	defaultCRDTimeout = 2 * time.Minute
)

// SetupController defines operations that should be run once to completion,
// typically at autopilot startup.
type SetupController interface {
	Run(ctx context.Context) error
}

type setupController struct {
	log           *logrus.Entry
	clientFactory apcli.FactoryInterface
	k0sDataDir    string
}

var _ SetupController = (*setupController)(nil)

// NewSetupController creates a `SetupController`
func NewSetupController(logger *logrus.Entry, cf apcli.FactoryInterface, k0sDataDir string) SetupController {
	return &setupController{
		log:           logger.WithField("controller", "setup"),
		clientFactory: cf,
		k0sDataDir:    k0sDataDir,
	}
}

// Run will go through all of the required setup operations that are required for autopilot.
// This effectively replaces the manifest concept used in k0s, as there is no guarantee that
// autopilot has access to the k0s file-system, or even if k0s is used at all.
func (sc *setupController) Run(ctx context.Context) error {
	logger := sc.log.WithField("component", "setup")

	logger.Infof("Applying embedded CRDs")
	if err := applyManifestCRDsWithWait(ctx, logger, sc.clientFactory, sc.k0sDataDir); err != nil {
		return fmt.Errorf("unable to extract embedded CRDs: %w", err)
	}

	logger.Infof("Creating namespace '%s'", apconst.AutopilotNamespace)
	if _, err := createNamespace(ctx, sc.clientFactory, apconst.AutopilotNamespace); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create required namespace '%s'", apconst.AutopilotNamespace)
		}
	}

	hostname, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname for signal node setup: %w", err)
	}

	logger.Infof("Using effective hostname = '%v'", hostname)

	logger.Infof("Creating controlnode '%s'", hostname)
	if err := sc.createControlNode(ctx, sc.clientFactory, hostname); err != nil {
		return fmt.Errorf("unable to create controlnode '%s': %w", hostname, err)
	}

	return nil
}

// createNamespace creates a namespace with the provided name
func createNamespace(ctx context.Context, cf apcli.FactoryInterface, name string) (*v1.Namespace, error) {
	client, err := cf.GetClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create obtain a kube client: %w", err)
	}

	namespace := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return client.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
}

// createControlNode creates a new control node, ignoring errors if one already exists
// for this physical host.
func (sc *setupController) createControlNode(ctx context.Context, cf apcli.FactoryInterface, name string) error {
	logger := sc.log.WithField("component", "setup")
	client, err := sc.clientFactory.GetAutopilotClient()
	if err != nil {
		return err
	}

	// Create the ControlNode object if needed
	node, err := client.AutopilotV1beta2().ControlNodes().Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		logger.Infof("ControlNode '%s' not found, creating", name)
		node = &apv1beta2.ControlNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				// Create the usual os and arch labels as this describes a controller node
				Labels: map[string]string{
					v1.LabelHostname:   name,
					v1.LabelOSStable:   runtime.GOOS,
					v1.LabelArchStable: runtime.GOARCH,
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

	addresses, err := getControlNodeAddresses(name)
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

func getControlNodeAddresses(hostname string) ([]v1.NodeAddress, error) {
	addresses := []v1.NodeAddress{}
	apiAddress, err := getControllerAPIAddress()
	if err != nil {
		return addresses, err
	}
	addresses = append(addresses, v1.NodeAddress{
		Type:    v1.NodeInternalIP,
		Address: apiAddress,
	})

	addresses = append(addresses, v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: hostname,
	})

	return addresses, nil
}

func getControllerAPIAddress() (string, error) {
	status, err := k0sinstall.GetStatusInfo(DefaultK0sStatusSocketPath)
	if err != nil {
		return "", err
	}

	return status.ClusterConfig.Spec.API.Address, nil
}

// applyManifestCRDsWithWait iterates over all of the embedded CRDs, applies them to the k0s
// manifest directory, and waits for them to be realized. In the event of a failure to realize,
// or timeout, an error will be returned.
func applyManifestCRDsWithWait(ctx context.Context, logger *logrus.Entry, cf apcli.FactoryInterface, k0sDataDir string) error {
	autopilotManifestDir := path.Join(k0sDataDir, apconst.K0sManifestSubDir, apconst.AutopilotName)
	if _, err := os.Stat(autopilotManifestDir); os.IsNotExist(err) {
		if err := os.Mkdir(autopilotManifestDir, 0755); err != nil {
			return err
		}
	}

	crds, err := apem.LoadCustomResourceDefinitions()
	if err != nil {
		return err
	}

	client, err := cf.GetExtensionClient()
	if err != nil {
		return err
	}

	for name, manifest := range crds {
		manifestFilename := path.Join(autopilotManifestDir, fmt.Sprintf("%s.yaml", name))
		tw := aptw.TemplateWriter{
			Name:     name,
			Template: string(manifest),
			Data:     nil,
			Path:     manifestFilename,
		}

		if err := tw.Write(); err != nil {
			return fmt.Errorf("unable to write CRD manifest to '%s': %w", manifestFilename, err)
		}

		logger.Infof("Successfully wrote CRD '%s' as '%s'", name, manifestFilename)
		logger.Infof("Waiting for CRD '%s' to be realized (timeout = %v)", name, defaultCRDTimeout)

		timestamp := time.Now()
		if _, err := apcomm.WaitForCRDByName(ctx, client, name, defaultCRDTimeout); err != nil {
			return fmt.Errorf("unable to wait for CRD '%s': %w", name, err)
		}

		logger.Infof("Finished waiting for CRD '%s' (actual = %v)", name, time.Since(timestamp))
	}

	return nil
}
