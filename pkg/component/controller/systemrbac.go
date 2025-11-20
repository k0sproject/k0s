/*
Copyright 2020 k0s authors

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
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"io"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

const SystemRBACStackName = "bootstraprbac"

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
	Clients          kubernetes.ClientFactoryInterface
	ExcludeAutopilot bool
}

var _ manager.Component = (*SystemRBAC)(nil)

// Applies the system RBAC manifests to the cluster.
func (s *SystemRBAC) Init(ctx context.Context) error {
	in := io.Reader(bytes.NewReader(systemRBAC))
	if !s.ExcludeAutopilot {
		in = io.MultiReader(in, bytes.NewReader(apSystemRBAC))
	}

	infos, err := resource.NewLocalBuilder().
		Unstructured().
		Stream(in, SystemRBACStackName).
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
	if err := retry.Do(
		func() error {
			stack := applier.Stack{
				Name:      SystemRBACStackName,
				Resources: resources,
				Clients:   s.Clients,
			}
			lastErr := stack.Apply(ctx, true)
			return lastErr
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(attempt uint, err error) {
			logrus.WithFields(logrus.Fields{
				"component": constant.SystemRBACComponentName,
				"stack":     SystemRBACStackName,
				"attempt":   attempt + 1,
			}).WithError(err).Debug("Failed to apply stack, retrying after backoff")
		}),
	); err != nil {
		return fmt.Errorf("failed to apply system RBAC stack: %w", cmp.Or(lastErr, err))
	}

	return nil
}

func (s *SystemRBAC) Start(context.Context) error { return nil }
func (s *SystemRBAC) Stop() error                 { return nil }

//go:embed systemrbac.yaml
var systemRBAC []byte

//go:embed systemrbac-ap.yaml
var apSystemRBAC []byte
