// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"text/template"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/static"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	apiretry "k8s.io/client-go/util/retry"

	"github.com/Masterminds/sprig"
	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const HelmExtensionStackName = "helm"

// ExtensionsController reconciles Helm extension repositories and charts, and
// keeps the Helm stack up to date as long as leadership is held.
type ExtensionsController struct {
	L             *logrus.Entry
	helm          *helm.Commands
	clientFactory kubeutil.ClientFactoryInterface
	leaderElector leaderelector.Interface
	stop          func()
	helmConfig    value.Latest[*k0sv1beta1.HelmExtensions]
}

var _ manager.Component = (*ExtensionsController)(nil)
var _ manager.Reconciler = (*ExtensionsController)(nil)

// NewExtensionsController builds new HelmAddons
func NewExtensionsController(k0sVars *config.CfgVars, kubeClientFactory kubeutil.ClientFactoryInterface, leaderElector leaderelector.Interface) *ExtensionsController {
	return &ExtensionsController{
		L:             logrus.WithFields(logrus.Fields{"component": "extensions_controller"}),
		helm:          helm.NewCommands(k0sVars),
		clientFactory: kubeClientFactory,
		leaderElector: leaderElector,
	}
}

const (
	namespaceToWatch = metav1.NamespaceSystem
)

// Run runs the extensions controller
func (ec *ExtensionsController) Reconcile(ctx context.Context, clusterConfig *k0sv1beta1.ClusterConfig) error {
	helmConfig := clusterConfig.Spec.Extensions.Helm.DeepCopy()
	if helmConfig == nil {
		helmConfig = new(k0sv1beta1.HelmExtensions)
	}
	ec.helmConfig.Set(helmConfig)
	return nil
}

func (ec *ExtensionsController) reconcileConfig(ctx context.Context, stackReconciledOnce chan<- struct{}) {
	var (
		currentRepositories k0sv1beta1.RepositoriesSettings
		currentCharts       k0sv1beta1.ChartsSettings
	)

	for {
		config, configChanged := ec.helmConfig.Peek()
		leaderStatus, statusChanged := ec.leaderElector.CurrentStatus()

		var retry <-chan time.Time
		if config != nil {
			var fail bool

			if reflect.DeepEqual(currentRepositories, config.Repositories) {
				ec.L.Debug("Helm repositories unchanged")
			} else if ec.reconcileRepositories(config.Repositories) {
				currentRepositories = config.Repositories
				ec.L.Info("Reconciled Helm repositories")
			} else {
				fail = true
			}

			if leaderStatus == leaderelection.StatusLeading {
				if reflect.DeepEqual(currentCharts, config.Charts) {
					ec.L.Debug("Helm charts unchanged")
				} else {
					ctx, cancel := context.WithCancelCause(ctx)
					go func() {
						select {
						case <-statusChanged:
							cancel(leaderelection.ErrLostLead)
						case <-ctx.Done():
						}
					}()

					if ec.reconcileStack(ctx, config.Charts) {
						currentCharts = config.Charts
						ec.L.Info("Reconciled ", HelmExtensionStackName, " stack")
						if stackReconciledOnce != nil {
							close(stackReconciledOnce)
							stackReconciledOnce = nil
						}
					} else {
						fail = true
					}
				}
			}

			if fail {
				retry = time.After(30 * time.Second)
			}
		}

		select {
		case <-configChanged:
			ec.L.Debug("Processing configuration change")

		case <-statusChanged:
			ec.L.Debug("Processing leader change")

		case <-retry:
			ec.L.Info("Retrying configuration reconciliation")

		case <-ctx.Done():
			return
		}
	}
}

func (ec *ExtensionsController) reconcileRepositories(repos k0sv1beta1.RepositoriesSettings) bool {
	var fail bool
	for _, repo := range repos {
		if err := ec.addRepo(repo); err != nil {
			fail = true
			ec.L.WithError(err).Error("Failed to reconcile Helm repository ", repo.URL)
		}
	}
	return !fail
}

func (ec *ExtensionsController) reconcileStack(ctx context.Context, charts k0sv1beta1.ChartsSettings) bool {
	var fail bool

	resources, err := applier.ReadUnstructuredDir(static.CRDs, HelmExtensionStackName)
	if err != nil {
		ec.L.WithError(err).Error("Failed to fetch Helm CRDs")
		return false
	}

	chartTemplate := template.Must(template.New("addon_crd_manifest").Funcs(sprig.TxtFuncMap()).Parse(chartCrdTemplate))
	for _, chart := range charts {
		var rendered bytes.Buffer
		if err := chartTemplate.Execute(&rendered, chart); err != nil {
			ec.L.WithError(err).Error("Failed to render Helm chart manifest ", chart.Name)
			return false
		}

		var object unstructured.Unstructured
		if err := yaml.Unmarshal(rendered.Bytes(), &object.Object); err != nil {
			ec.L.WithError(err).Error("Failed to render Helm chart manifest ", chart.Name)
			return false
		}

		resources = append(resources, &object)
	}

	if err := applier.ApplyStack(ctx, ec.clientFactory, resources, HelmExtensionStackName); err != nil {
		fail = true
		ec.L.WithError(err).Error("Failed to apply ", HelmExtensionStackName, " stack")
	}

	return !fail
}

type ChartReconciler struct {
	client.Client
	helm          *helm.Commands
	leaderElector leaderelector.Interface
	L             *logrus.Entry
}

func (cr *ChartReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if !cr.leaderElector.IsLeader() {
		return reconcile.Result{}, nil
	}
	cr.L.Tracef("Got helm chart reconciliation request: %s", req)
	defer cr.L.Tracef("Finished processing helm chart reconciliation request: %s", req)

	var chartInstance helmv1beta1.Chart

	if err := cr.Get(ctx, req.NamespacedName, &chartInstance); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !chartInstance.DeletionTimestamp.IsZero() {
		cr.L.Debugf("Uninstall reconciliation request: %s", req)
		// uninstall chart
		if err := cr.uninstall(ctx, chartInstance); err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				return reconcile.Result{}, fmt.Errorf("can't uninstall chart: %w", err)
			}

			cr.L.Debugf("No Helm release found for chart %s, assuming it has already been uninstalled", req)
		}

		if err := removeFinalizer(ctx, cr.Client, &chartInstance); err != nil {
			return reconcile.Result{}, fmt.Errorf("while trying to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}
	cr.L.Debugf("Install or update reconciliation request: %s", req)
	if err := cr.updateOrInstallChart(ctx, chartInstance); err != nil {
		return reconcile.Result{Requeue: true}, fmt.Errorf("can't update or install chart: %w", err)
	}

	cr.L.Debugf("Installed or updated reconciliation request: %s", req)
	return reconcile.Result{}, nil
}

func (cr *ChartReconciler) uninstall(ctx context.Context, chart helmv1beta1.Chart) error {
	if err := cr.helm.UninstallRelease(ctx, chart.Status.ReleaseName, chart.Status.Namespace); err != nil {
		return fmt.Errorf("can't uninstall release `%s/%s`: %w", chart.Status.Namespace, chart.Status.ReleaseName, err)
	}
	return nil
}

func removeFinalizer(ctx context.Context, c client.Client, chart *helmv1beta1.Chart) error {
	idx := slices.Index(chart.Finalizers, finalizerName)
	if idx < 0 {
		return nil
	}

	path := fmt.Sprintf("/metadata/finalizers/%d", idx)
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value,omitempty"`
	}{
		{"test", path, finalizerName},
		{"remove", path, ""},
	})
	if err != nil {
		return err
	}

	return c.Patch(ctx, chart, client.RawPatch(types.JSONPatchType, patch))
}

const defaultTimeout = 10 * time.Minute

func (cr *ChartReconciler) updateOrInstallChart(ctx context.Context, chart helmv1beta1.Chart) error {
	var err error
	var chartRelease *release.Release
	timeout, err := time.ParseDuration(chart.Spec.Timeout)
	if err != nil {
		cr.L.Tracef("Can't parse `%s` as time.Duration, using default timeout `%s`", chart.Spec.Timeout, defaultTimeout)
		timeout = defaultTimeout
	}
	if timeout == 0 {
		cr.L.Tracef("Using default timeout `%s`, failed to parse `%s`", defaultTimeout, chart.Spec.Timeout)
		timeout = defaultTimeout
	}
	defer func() {
		if err == nil {
			return
		}
		if err := apiretry.RetryOnConflict(apiretry.DefaultRetry, func() error {
			return cr.updateStatus(ctx, chart, chartRelease, err)
		}); err != nil {
			cr.L.WithError(err).Error("Failed to update status for chart release, give up", chart.Name)
		}
	}()
	if chart.Status.ReleaseName == "" {
		// new chartRelease
		cr.L.Tracef("Start update or install %s", chart.Spec.ChartName)
		chartRelease, err = cr.helm.InstallChart(ctx,
			chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Spec.ReleaseName,
			chart.Spec.Namespace,
			chart.Spec.YamlValues(),
			timeout,
		)
		if err != nil {
			return fmt.Errorf("can't reconcile installation for %q: %w", chart.GetName(), err)
		}
	} else {
		if !cr.chartNeedsUpgrade(chart) {
			return nil
		}
		// update
		chartRelease, err = cr.helm.UpgradeChart(ctx,
			chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Status.ReleaseName,
			chart.Status.Namespace,
			chart.Spec.YamlValues(),
			timeout,
			chart.Spec.ShouldForceUpgrade(),
		)
		if err != nil {
			return fmt.Errorf("can't reconcile upgrade for %q: %w", chart.GetName(), err)
		}
	}
	if err := apiretry.RetryOnConflict(apiretry.DefaultRetry, func() error {
		return cr.updateStatus(ctx, chart, chartRelease, nil)
	}); err != nil {
		cr.L.WithError(err).Error("Failed to update status for chart release, give up", chart.Name)
	}
	return nil
}

func (cr *ChartReconciler) chartNeedsUpgrade(chart helmv1beta1.Chart) bool {
	return chart.Status.Namespace != chart.Spec.Namespace ||
		chart.Status.ReleaseName != chart.Spec.ReleaseName ||
		chart.Status.Version != chart.Spec.Version ||
		chart.Status.ValuesHash != chart.Spec.HashValues()
}

// updateStatus updates the status of the chart with the given release information. This function
// starts by fetching an updated version of the chart from the api as the install may take a while
// to complete and the chart may have been updated in the meantime. If returns the error returned
// by the Update operation. Moreover, if the chart has indeed changed in the meantime we already
// have an event for it so we will see it again soon.
func (cr *ChartReconciler) updateStatus(ctx context.Context, chart helmv1beta1.Chart, chartRelease *release.Release, err error) error {
	nsn := types.NamespacedName{Namespace: chart.Namespace, Name: chart.Name}
	var updchart helmv1beta1.Chart
	if err := cr.Get(ctx, nsn, &updchart); err != nil {
		return fmt.Errorf("can't get updated version of chart %s: %w", chart.Name, err)
	}
	chart.Spec.YamlValues() // XXX what is this function for ?
	if chartRelease != nil {
		updchart.Status.ReleaseName = chartRelease.Name
		updchart.Status.Version = chartRelease.Chart.Metadata.Version
		updchart.Status.AppVersion = chartRelease.Chart.AppVersion()
		updchart.Status.Revision = int64(chartRelease.Version)
		updchart.Status.Namespace = chartRelease.Namespace
	}
	updchart.Status.Updated = time.Now().String()
	updchart.Status.Error = ""
	if err != nil {
		updchart.Status.Error = err.Error()
	}
	updchart.Status.ValuesHash = chart.Spec.HashValues()
	if updErr := cr.Client.Status().Update(ctx, &updchart); updErr != nil {
		cr.L.WithError(updErr).Error("Failed to update status for chart release", chart.Name)
		return updErr
	}
	return nil
}

func (ec *ExtensionsController) addRepo(repo k0sv1beta1.Repository) error {
	return ec.helm.AddRepository(repo)
}

const chartCrdTemplate = `
---
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: ` + metav1.NamespaceSystem + `
  finalizers:
    - ` + finalizerName + `
spec:
  chartName: {{ .ChartName }}
  releaseName: {{ .Name }}
  timeout: {{ .Timeout.Duration }}
  values: |
{{ .Values | nindent 4 }}
  version: {{ .Version }}
  namespace: {{ .TargetNS }}
{{- if ne .ForceUpgrade nil }}
  forceUpgrade: {{ .ForceUpgrade }}
{{- end }}
`

const finalizerName = "helm.k0sproject.io/uninstall-helm-release"

// Init
func (ec *ExtensionsController) Init(_ context.Context) error {
	return nil
}

// Start
func (ec *ExtensionsController) Start(context.Context) error {
	ec.L.Debug("Starting")

	mgr, err := ec.instantiateManager()
	if err != nil {
		return fmt.Errorf("can't instantiate controller-runtime manager: %w", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	stackReconciledOnce := make(chan struct{})

	wg.Go(func() { ec.reconcileConfig(ctx, stackReconciledOnce) })
	wg.Go(func() {
		leaderelection.RunLeaderTasks(ctx, ec.leaderElector.CurrentStatus, func(ctx context.Context) {
			select {
			case <-stackReconciledOnce:
			case <-ctx.Done():
				return
			}

			ec.L.Info("Running controller-runtime manager")
			if err := mgr.Start(ctx); err != nil {
				ec.L.WithError(err).Error("Failed to run controller-runtime manager")
			} else {
				ec.L.Info("Controller-runtime manager exited")
			}
		})

	})

	ec.stop = func() {
		cancel()
		wg.Wait()
	}

	return nil
}

func (ec *ExtensionsController) instantiateManager() (crman.Manager, error) {
	clientConfig, err := ec.clientFactory.GetRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}

	mgr, err := controllerruntime.NewManager(clientConfig, crman.Options{
		Scheme: k0sscheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Logger:     logrusr.New(ec.L),
		Controller: ctrlconfig.Controller{},
	})
	if err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}

	if err := builder.
		ControllerManagedBy(mgr).
		Named("chart").
		For(&helmv1beta1.Chart{},
			builder.WithPredicates(predicate.And(
				predicate.GenerationChangedPredicate{},
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					return object.GetNamespace() == namespaceToWatch
				}),
			),
			),
		).
		Complete(&ChartReconciler{
			Client:        mgr.GetClient(),
			leaderElector: ec.leaderElector, // TODO: drop in favor of controller-runtime lease manager?
			helm:          ec.helm,
			L:             ec.L.WithField("extensions_type", "helm"),
		}); err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	return mgr, nil
}

// Stop
func (ec *ExtensionsController) Stop() error {
	if ec.stop != nil {
		ec.stop()
	}
	ec.L.Debug("Stopped extensions controller")
	return nil
}
