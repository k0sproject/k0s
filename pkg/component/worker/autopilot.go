//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

var _ manager.Component = (*Autopilot)(nil)

type Autopilot struct {
	K0sVars     *config.CfgVars
	CertManager *CertificateManager

	clientFactory apcli.FactoryInterface
}

func (a *Autopilot) Init(ctx context.Context) error {
	return nil
}

func (a *Autopilot) Start(ctx context.Context) error {
	log := logrus.WithField("component", "autopilot")

	kinds, err := getAutopilotKinds()
	if err != nil {
		return err
	}

	var lastErr error
	if err := retry.Do(
		func() (err error) {
			defer func() { lastErr = err }()

			restConfig, err := a.CertManager.GetRestConfig(ctx)
			if err != nil {
				return err
			}

			a.clientFactory = &apcli.ClientFactory{ClientFactoryInterface: &kubernetes.ClientFactory{
				LoadRESTConfig: func() (*rest.Config, error) { return restConfig, nil },
			}}

			// We need to wait until all autopilot CRDs are established.
			kinds := slices.Clone(kinds) // take a copy to avoid side effects
			if err := waitUntilCRDsEstablished(ctx, a.clientFactory, kinds); err != nil {
				return fmt.Errorf("while waiting for Autopilot CRDs %v to become established: %w", kinds, err)
			}

			return nil
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(attempt uint, err error) {
			log.WithField("attempt", attempt+1).WithError(err).Debug("Retrying after backoff")
		}),
	); err != nil {
		return fmt.Errorf("failed to initialize autopilot: %w", cmp.Or(lastErr, err))
	}

	autopilotRoot, err := apcont.NewRootWorker(aproot.RootConfig{
		KubeConfig:          a.K0sVars.KubeletAuthConfigPath,
		K0sDataDir:          a.K0sVars.DataDir,
		Mode:                "worker",
		ManagerPort:         8899,
		MetricsBindAddr:     "0",
		HealthProbeBindAddr: "0",
	}, log, a.clientFactory)
	if err != nil {
		return fmt.Errorf("failed to create autopilot worker: %w", err)
	}

	go func() {
		if err := autopilotRoot.Run(ctx); err != nil {
			logrus.WithError(err).Error("Error running autopilot")

			// TODO: We now have a service with nothing running.. now what?
		}
	}()

	return nil
}

// Stop stops Autopilot
func (a *Autopilot) Stop() error {
	return nil
}

// Gathers all kinds in the autopilot API group.
func getAutopilotKinds() ([]string, error) {
	var kinds []string

	gv := autopilotv1beta2.SchemeGroupVersion
	for kind := range k0sscheme.Scheme.KnownTypes(gv) {
		// For some reason, the scheme also returns types from core/v1. Filter
		// those out by only adding kinds which are _only_ in the autopilot
		// group, and not in some other group as well. The only way to get all
		// the GVKs for a certain type is by creating a new instance of that
		// type and then asking the scheme about it.
		obj, err := k0sscheme.Scheme.New(gv.WithKind(kind))
		if err != nil {
			return nil, err
		}
		gvks, _, err := k0sscheme.Scheme.ObjectKinds(obj)
		if err != nil {
			return nil, err
		}

		// Skip the kind if there's at least one GVK which is not in the
		// autopilot group
		if !slices.ContainsFunc(gvks, func(gvk schema.GroupVersionKind) bool {
			return gvk.Group != autopilotv1beta2.GroupName
		}) {
			kinds = append(kinds, kind)
		}
	}

	slices.Sort(kinds) // for cosmetic purposes
	return kinds, nil
}

func waitUntilCRDsEstablished(ctx context.Context, clientFactory apcli.FactoryInterface, kinds []string) error {
	client, err := clientFactory.GetExtensionClient()
	if err != nil {
		return err
	}

	// Watch all the CRDs until all the required ones are established.
	log := logrus.WithField("component", "autopilot")
	return watch.CRDs(client.CustomResourceDefinitions()).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryAfter, e := watch.IsRetryable(err); e == nil {
				log.WithError(err).Info(
					"Transient error while watching for CRDs",
					", starting over after ", retryAfter, " ...",
				)
				return retryAfter, nil
			}

			retryAfter := 10 * time.Second
			log.WithError(err).Error(
				"Error while watching CRDs",
				", starting over after ", retryAfter, " ...",
			)
			return retryAfter, nil
		}).
		Until(ctx, func(item *apiextensionsv1.CustomResourceDefinition) (bool, error) {
			if item.Spec.Group != autopilotv1beta2.GroupName {
				return false, nil // Not an autopilot CRD.
			}

			// Find the established status for the CRD.
			var established apiextensionsv1.ConditionStatus
			for _, cond := range item.Status.Conditions {
				if cond.Type == apiextensionsv1.Established {
					established = cond.Status
					break
				}
			}

			if established != apiextensionsv1.ConditionTrue {
				return false, nil // CRD not yet established.
			}

			// Remove the CRD's (list) kind from the list.
			kinds = slices.DeleteFunc(kinds, func(kind string) bool {
				return kind == item.Spec.Names.Kind || kind == item.Spec.Names.ListKind
			})

			// If the list is empty, all required CRDs are established.
			return len(kinds) < 1, nil
		})
}
