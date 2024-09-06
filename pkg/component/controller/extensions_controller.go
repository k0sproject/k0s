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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/bombsimon/logrusr/v4"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	apiretry "k8s.io/client-go/util/retry"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Helm watch for Chart crd
type ExtensionsController struct {
	L             *logrus.Entry
	helm          *helm.Commands
	kubeConfig    string
	leaderElector leaderelector.Interface
	manifestsDir  string
	startChan     chan struct{}
	mux           sync.Mutex
	mgr           crman.Manager
	mgrCancelFn   context.CancelFunc
}

var _ manager.Component = (*ExtensionsController)(nil)
var _ manager.Reconciler = (*ExtensionsController)(nil)

// NewExtensionsController builds new HelmAddons
func NewExtensionsController(k0sVars *config.CfgVars, kubeClientFactory kubeutil.ClientFactoryInterface, leaderElector leaderelector.Interface) *ExtensionsController {
	return &ExtensionsController{
		L:             logrus.WithFields(logrus.Fields{"component": "extensions_controller"}),
		helm:          helm.NewCommands(k0sVars),
		kubeConfig:    k0sVars.AdminKubeConfigPath,
		leaderElector: leaderElector,
		manifestsDir:  filepath.Join(k0sVars.ManifestsDir, "helm"),
	}
}

const (
	namespaceToWatch = "kube-system"
)

// Run runs the extensions controller
func (ec *ExtensionsController) Reconcile(ctx context.Context, clusterConfig *k0sv1beta1.ClusterConfig) error {
	ec.L.Info("Extensions reconciliation started")
	defer ec.L.Info("Extensions reconciliation finished")
	return ec.reconcileHelmExtensions(clusterConfig.Spec.Extensions.Helm)
}

// reconcileHelmExtensions creates instance of Chart CR for each chart of the config file
// it also reconciles repositories settings
// the actual helm install/update/delete management is done by ChartReconciler structure
func (ec *ExtensionsController) reconcileHelmExtensions(helmSpec *k0sv1beta1.HelmExtensions) error {
	if helmSpec == nil {
		return nil
	}

	var errs []error
	for _, repo := range helmSpec.Repositories {
		if err := ec.addRepo(repo); err != nil {
			errs = append(errs, fmt.Errorf("can't init repository %q: %w", repo.URL, err))
		}
	}

	var fileNamesToKeep []string
	for _, chart := range helmSpec.Charts {
		fileName := chartManifestFileName(&chart)
		fileNamesToKeep = append(fileNamesToKeep, fileName)

		path, err := ec.writeChartManifestFile(chart, fileName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't write file for Helm chart manifest %q: %w", chart.ChartName, err))
			continue
		}

		ec.L.Infof("Wrote Helm chart manifest file %q", path)
	}

	if err := filepath.WalkDir(ec.manifestsDir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case !entry.Type().IsRegular():
			ec.L.Debugf("Keeping %v as it is not a regular file", entry)
		case slices.Contains(fileNamesToKeep, entry.Name()):
			ec.L.Debugf("Keeping %v as it belongs to a known Helm extension", entry)
		case !isChartManifestFileName(entry.Name()):
			ec.L.Debugf("Keeping %v as it is not a Helm chart manifest file", entry)
		default:
			if err := os.Remove(path); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					errs = append(errs, fmt.Errorf("failed to remove Helm chart manifest file, the Chart resource will remain in the cluster: %w", err))
				}
			} else {
				ec.L.Infof("Removed Helm chart manifest file %q", path)
			}
		}

		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("failed to walk Helm chart manifest directory: %w", err))
	}

	return errors.Join(errs...)
}

func (ec *ExtensionsController) writeChartManifestFile(chart k0sv1beta1.Chart, fileName string) (string, error) {
	tw := templatewriter.TemplateWriter{
		Path:     filepath.Join(ec.manifestsDir, fileName),
		Name:     "addon_crd_manifest",
		Template: chartCrdTemplate,
		Data: struct {
			k0sv1beta1.Chart
			Finalizer string
		}{
			Chart:     chart,
			Finalizer: finalizerName,
		},
	}
	if err := tw.Write(); err != nil {
		return "", err
	}
	return tw.Path, nil
}

// Determines the file name to use when storing a chart as a manifest on disk.
func chartManifestFileName(c *k0sv1beta1.Chart) string {
	return fmt.Sprintf("%d_helm_extension_%s.yaml", c.Order, c.Name)
}

// Determines if the given file name is in the format for chart manifest file names.
func isChartManifestFileName(fileName string) bool {
	return regexp.MustCompile(`^-?[0-9]+_helm_extension_.+\.yaml$`).MatchString(fileName)
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

	if err := cr.Client.Get(ctx, req.NamespacedName, &chartInstance); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !chartInstance.ObjectMeta.DeletionTimestamp.IsZero() {
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

const defaultTimeout = time.Duration(10 * time.Minute)

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
	return !(chart.Status.Namespace == chart.Spec.Namespace &&
		chart.Status.ReleaseName == chart.Spec.ReleaseName &&
		chart.Status.Version == chart.Spec.Version &&
		chart.Status.ValuesHash == chart.Spec.HashValues())
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
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: "kube-system"
  finalizers:
    - {{ .Finalizer }}
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
func (ec *ExtensionsController) Start(ctx context.Context) error {
	ec.L.Debug("Starting")
	ec.startChan = make(chan struct{}, 1)

	// Do the first validation before setting callbacks
	var err error
	ec.mgr, err = ec.instantiateManager(ctx)
	if err != nil {
		ec.L.WithError(err).Error("Can't instantiate controller-runtime manager")
		return err
	}

	ec.leaderElector.AddLostLeaseCallback(ec.leaseLost)

	ec.leaderElector.AddAcquiredLeaseCallback(ec.leaseAcquired)

	// It's possible that by the time we added the callback, we are already the leader,
	// If this is true the callback will not be called, so we need to check if we are
	// the leader and notify the channel manually
	if ec.leaderElector.IsLeader() {
		ec.leaseAcquired()
	}

	go ec.watchStartChan()
	return nil
}

func (ec *ExtensionsController) leaseLost() {
	ec.mux.Lock()
	defer ec.mux.Unlock()
	ec.L.Warn("Lost leader lease, stopping controller-manager")

	mgrCancelFn := ec.mgrCancelFn
	if mgrCancelFn != nil {
		mgrCancelFn()
	}
	ec.mgr = nil
}

func (ec *ExtensionsController) watchStartChan() {
	ec.L.Debug("Watching start channel")
	for range ec.startChan {
		ec.L.Info("Acquired leader lease")
		ec.mux.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		// If there is a previous cancel func, call it
		if ec.mgrCancelFn != nil {
			ec.mgrCancelFn()
		}
		ec.mgrCancelFn = cancel
		if ec.mgr == nil {
			ec.L.Info("Instantiating controller-runtime manager")
			var err error
			ec.mgr, err = ec.instantiateManager(ctx)
			if err != nil {
				ec.L.WithError(err).Error("Can't instantiate controller-runtime manager")
				ec.mux.Unlock()
				return
			}
		}
		ec.mux.Unlock()
		ec.startControllerManager(ctx)
	}
	ec.L.Info("Start channel closed, stopping controller-manager")
}

func (ec *ExtensionsController) leaseAcquired() {
	ec.L.Info("Acquired leader lease")
	select {
	case ec.startChan <- struct{}{}:
	default:
	}
}

func (ec *ExtensionsController) instantiateManager(ctx context.Context) (crman.Manager, error) {
	clientConfig, err := clientcmd.BuildConfigFromFlags("", ec.kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	gk := schema.GroupKind{
		Group: helmv1beta1.GroupName,
		Kind:  "Chart",
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
	if err := retry.Do(func() error {
		_, err := mgr.GetRESTMapper().RESTMapping(gk)
		if err != nil {
			ec.L.Warn("Extensions CRD is not yet ready, waiting before starting ExtensionsController")
			return err
		}
		ec.L.Info("Extensions CRD is ready, going nuts")
		return nil
	}, retry.Context(ctx)); err != nil {
		return nil, fmt.Errorf("can't start ExtensionsReconciler, helm CRD is not registered, check CRD registration reconciler: %w", err)
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

func (ec *ExtensionsController) startControllerManager(ctx context.Context) {
	go func() {
		ec.L.Info("Starting controller-manager")
		if err := ec.mgr.Start(ctx); err != nil {
			ec.L.WithError(err).Error("Controller manager working loop exited")
		}
	}()
}

// Stop
func (ec *ExtensionsController) Stop() error {
	ec.L.Info("Stopping extensions controller")
	// We have no guarantees on concurrency here, so use mutex
	ec.mux.Lock()
	mgrCancelFn := ec.mgrCancelFn
	ec.mux.Unlock()
	if mgrCancelFn != nil {
		mgrCancelFn()
	}
	close(ec.startChan)
	ec.L.Debug("Stopped extensions controller")
	return nil
}
