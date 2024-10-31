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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	k0sv1beta1client "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/clusterconfig"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/static"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

const ClusterConfigStackName = "api-config"

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	KubeClientFactory kubernetes.ClientFactoryInterface

	log          *logrus.Entry
	reconciler   manager.Reconciler
	configSource clusterconfig.ConfigSource
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(reconciler manager.Reconciler, kubeClientFactory kubernetes.ClientFactoryInterface, configSource clusterconfig.ConfigSource) *ClusterConfigReconciler {
	return &ClusterConfigReconciler{
		KubeClientFactory: kubeClientFactory,
		log:               logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
		configSource:      configSource,
		reconciler:        reconciler,
	}
}

func (r *ClusterConfigReconciler) Init(context.Context) error { return nil }

func (r *ClusterConfigReconciler) Start(ctx context.Context) error {
	go func() {
		statusCtx := ctx
		r.log.Debug("start listening changes from config source")
		for {
			select {
			case cfg, ok := <-r.configSource.ResultChan():
				if !ok {
					// Recv channel close, we can stop now
					r.log.Debug("config source closed channel")
					return
				}
				err := errors.Join(cfg.Validate()...)
				if err != nil {
					err = fmt.Errorf("failed to validate cluster configuration: %w", err)
				} else {
					err = r.reconciler.Reconcile(ctx, cfg)
				}
				r.reportStatus(statusCtx, cfg, err)
				if err != nil {
					r.log.WithError(err).Error("Failed to reconcile cluster configuration")
				} else {
					r.log.Debug("Successfully reconciled cluster configuration")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Stop stops
func (r *ClusterConfigReconciler) Stop() error {
	// Nothing really to stop, the main ConfigSource "watch" channel go-routine is stopped
	// via the main Context's Done channel in the Run function
	return nil
}

func (r *ClusterConfigReconciler) reportStatus(ctx context.Context, config *k0sv1beta1.ClusterConfig, reconcileError error) {
	hostname, err := os.Hostname()
	if err != nil {
		r.log.Error("failed to get hostname:", err)
		hostname = ""
	}
	// TODO We need to design proper status field(s) to the cluster cfg object, now just send event
	client, err := r.KubeClientFactory.GetClient()
	if err != nil {
		r.log.Error("failed to get kube client:", err)
	}
	e := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "k0s.",
		},
		EventTime:      metav1.NowMicro(),
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		InvolvedObject: corev1.ObjectReference{
			Kind:            k0sv1beta1.ClusterConfigKind,
			Namespace:       config.Namespace,
			Name:            config.Name,
			UID:             config.UID,
			APIVersion:      k0sv1beta1.ClusterConfigAPIVersion,
			ResourceVersion: config.ResourceVersion,
		},
		Action:              "ConfigReconciling",
		ReportingController: "k0s-controller",
		ReportingInstance:   hostname,
	}
	if reconcileError != nil {
		e.Reason = "FailedReconciling"
		e.Message = reconcileError.Error()
		e.Type = corev1.EventTypeWarning
	} else {
		e.Reason = "SuccessfulReconcile"
		e.Message = "Successfully reconciled cluster config"
		e.Type = corev1.EventTypeNormal
	}
	_, err = client.CoreV1().Events(constant.ClusterConfigNamespace).Create(ctx, e, metav1.CreateOptions{})
	if err != nil {
		r.log.Error("failed to create event for config reconcile:", err)
	}
}

type ClusterConfigInitializer struct {
	log           logrus.FieldLogger
	clients       kubernetes.ClientFactoryInterface
	leaderElector leaderelector.Interface
	initialConfig *k0sv1beta1.ClusterConfig
}

// Init implements [manager.Component].
func (*ClusterConfigInitializer) Init(context.Context) error { return nil }

// Start implements [manager.Component].
func (i *ClusterConfigInitializer) Start(ctx context.Context) error {
	if err := i.ensureClusterConfigExistence(ctx); err != nil {
		return fmt.Errorf("failed to ensure the existence of the cluster configuration: %w", err)
	}
	return nil
}

// Stop implements [manager.Component].
func (*ClusterConfigInitializer) Stop() error { return nil }

func NewClusterConfigInitializer(clients kubernetes.ClientFactoryInterface, leaderElector leaderelector.Interface, initialConfig *k0sv1beta1.ClusterConfig) *ClusterConfigInitializer {
	return &ClusterConfigInitializer{
		log:           logrus.WithField("component", "clusterConfigInitializer"),
		clients:       clients,
		leaderElector: leaderElector,
		initialConfig: initialConfig,
	}
}

func (i *ClusterConfigInitializer) ensureClusterConfigExistence(ctx context.Context) (err error) {
	clientset, err := i.clients.GetK0sClient()
	if err != nil {
		return err
	}
	client := clientset.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)

	// We need to wait until the cluster configuration exists or we succeed in creating it.
	var stackApplied bool
	pollErr := wait.PollUntilContextTimeout(ctx, 1*time.Second, 20*time.Second, true, func(ctx context.Context) (bool, error) {
		if i.leaderElector.IsLeader() {
			if stackApplied {
				err = nil
			} else {
				err = i.applyAPIConfigStack(ctx)
			}
			if err == nil {
				stackApplied = true
				err = i.createClusterConfig(ctx, client)
				if err == nil {
					i.log.Debug("Cluster configuration created")
					return true, nil
				}
				if apierrors.IsAlreadyExists(err) {
					// An already existing configuration is just fine.
					i.log.Debug("Cluster configuration already exists")
					return true, nil
				}
			}
		} else {
			err = i.clusterConfigExists(ctx, client)
			if err == nil {
				i.log.Debug("Cluster configuration exists")
				return true, nil
			}
		}

		i.log.WithError(err).Debug("Failed to ensure the existence of the cluster configuration")
		return false, nil
	})

	if pollErr != nil {
		pollErr = cmp.Or(context.Cause(ctx), pollErr)
		if err != nil {
			return fmt.Errorf("%w (%w)", pollErr, err)
		}

		return pollErr
	}

	return nil
}

func (i *ClusterConfigInitializer) applyAPIConfigStack(ctx context.Context) error {
	rawCRD, err := fs.ReadFile(static.CRDs, "k0s/k0s.k0sproject.io_clusterconfigs.yaml")
	if err != nil {
		return err
	}

	var crd unstructured.Unstructured
	if err := yaml.Unmarshal(rawCRD, &crd); err != nil {
		return err
	}

	return (&applier.Stack{
		Name:      ClusterConfigStackName,
		Resources: []*unstructured.Unstructured{&crd},
		Clients:   i.clients,
	}).Apply(ctx, true)
}

func (i *ClusterConfigInitializer) clusterConfigExists(ctx context.Context, client k0sv1beta1client.ClusterConfigInterface) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := client.Get(ctx, constant.ClusterConfigObjectName, metav1.GetOptions{})
	return err
}

func (i *ClusterConfigInitializer) createClusterConfig(ctx context.Context, client k0sv1beta1client.ClusterConfigInterface) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	clusterWideConfig := i.initialConfig.GetClusterWideConfig().StripDefaults().CRValidator()
	_, err := client.Create(ctx, clusterWideConfig, metav1.CreateOptions{})
	return err
}
