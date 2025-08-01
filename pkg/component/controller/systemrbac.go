// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/sirupsen/logrus"
)

const SystemRBACStackName = "bootstraprbac"

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
	Clients kubernetes.ClientFactoryInterface
}

var _ manager.Component = (*SystemRBAC)(nil)

// Applies the system RBAC manifests to the cluster.
func (s *SystemRBAC) Init(ctx context.Context) error {
	infos, err := resource.NewLocalBuilder().
		Unstructured().
		Stream(bytes.NewReader(systemRBAC), SystemRBACStackName).
		Flatten().
		Do().
		Infos()
	if err != nil {
		return err
	}

	resources := make([]*unstructured.Unstructured, len(infos))
	for i := range infos {
		resources[i] = infos[i].Object.(*unstructured.Unstructured)
	}

	var lastErr error
	var attempt uint
	err = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		attempt++
		stack := applier.Stack{
			Name:      SystemRBACStackName,
			Resources: resources,
			Clients:   s.Clients,
		}
		lastErr = stack.Apply(ctx, true)
		if lastErr != nil {
			logrus.WithFields(logrus.Fields{
				"component": constant.SystemRBACComponentName,
				"stack":     SystemRBACStackName,
				"attempt":   attempt,
			}).WithError(lastErr).Debug("Failed to apply stack, retrying after backoff")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply system RBAC stack: %w", cmp.Or(lastErr, err))
	}

	return nil
}

func (s *SystemRBAC) Start(context.Context) error { return nil }
func (s *SystemRBAC) Stop() error                 { return nil }

//go:embed systemrbac.yaml
var systemRBAC []byte
