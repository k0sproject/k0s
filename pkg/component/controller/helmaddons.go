package controller

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/v1beta1"
	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// Helm watch for Chart crd
type ExtensionsController struct {
	saver         manifestsSaver
	L             *logrus.Entry
	helm          *helm.Commands
	kubeConfig    string
	leaderElector LeaderElector
}

var _ component.Component = &ExtensionsController{}
var _ component.ReconcilerComponent = &ExtensionsController{}

// NewExtensionsController builds new HelmAddons
func NewExtensionsController(s manifestsSaver, k0sVars constant.CfgVars, kubeClientFactory kubeutil.ClientFactoryInterface, leaderElector LeaderElector) *ExtensionsController {
	return &ExtensionsController{
		saver:         s,
		L:             logrus.WithFields(logrus.Fields{"component": "extensions_controller"}),
		helm:          helm.NewCommands(k0sVars),
		kubeConfig:    k0sVars.AdminKubeConfigPath,
		leaderElector: leaderElector,
	}
}

const (
	namespaceToWatch = "kube-system"
)

// Run runs the extensions controller
func (ec *ExtensionsController) Reconcile(ctx context.Context, clusterConfig *k0sAPI.ClusterConfig) error {
	ec.L.Info("Extensions reconcilation started")
	defer ec.L.Info("Extensions reconcilation finished")

	if err := ec.reconcileHelmExtensions(clusterConfig.Spec.Extensions.Helm); err != nil {
		return fmt.Errorf("can't reconcile helm based extensions: %v", err)
	}

	return nil
}

func (ec *ExtensionsController) reconcileHelmExtensions(helmSpec *k0sAPI.HelmExtensions) error {
	if helmSpec == nil {
		return nil
	}
	for _, repo := range helmSpec.Repositories {
		if err := ec.addRepo(repo); err != nil {
			return fmt.Errorf("can't init repository `%s`: %v", repo.URL, err)
		}
	}

	for _, addon := range helmSpec.Charts {
		tw := templatewriter.TemplateWriter{
			Name:     "addon_crd_manifest",
			Template: chartCrdTemplate,
			Data:     addon,
		}
		buf := bytes.NewBuffer([]byte{})
		if err := tw.WriteToBuffer(buf); err != nil {
			ec.L.WithError(err).Errorf("can't render helm addon crd template")
			return fmt.Errorf("can't create addon `%s`: %v", addon.ChartName, err)
		}
		if err := ec.saver.Save("addon_crd_manifest_"+addon.Name+".yaml", buf.Bytes()); err != nil {
			return fmt.Errorf("can't save addon CRD manifest: %v", err)
		}
	}
	return nil
}

type ChartReconciler struct {
	client.Client
	helm          *helm.Commands
	leaderElector LeaderElector
}

func (cr *ChartReconciler) InjectClient(c client.Client) error {
	cr.Client = c
	return nil
}

func (cr *ChartReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if !cr.leaderElector.IsLeader() {
		return reconcile.Result{}, nil
	}
	var chartInstance v1beta1.Chart

	if err := cr.Client.Get(ctx, req.NamespacedName, &chartInstance); err != nil {
		// how to uninstall? no meta information avaiable
		// probably through finalizer?
	}

	if err := cr.updateOrInstallChart(ctx, chartInstance); err != nil {
		return reconcile.Result{Requeue: true}, fmt.Errorf("can't update or install chart: %w", err)
	}
	return reconcile.Result{}, nil
}

func (cr *ChartReconciler) updateOrInstallChart(ctx context.Context, chart v1beta1.Chart) error {
	var err error
	var chartRelease *release.Release
	if chart.Status.ReleaseName == "" {
		// new chartRelease
		chartRelease, err = cr.helm.InstallChart(chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Spec.Namespace,
			chart.Spec.YamlValues())
		if err != nil {
			return fmt.Errorf("can't reconcile installation for `%s`: %v", chart.GetName(), err)
		}
	} else {
		// update
		chartRelease, err = cr.helm.UpgradeChart(chart.Spec.ChartName,
			chart.Status.Version,
			chart.Status.ReleaseName,
			chart.Status.Namespace,
			chart.Spec.YamlValues(),
		)
		if err != nil {
			return fmt.Errorf("can't reconcile upgrade for `%s`: %v", chart.GetName(), err)
		}
	}

	chart.Status.ReleaseName = chartRelease.Name
	chart.Status.Version = chartRelease.Chart.Metadata.Version
	chart.Status.AppVersion = chartRelease.Chart.AppVersion()
	chart.Status.Updated = time.Now().String()
	chart.Status.Revision = int64(chartRelease.Version)
	chart.Status.Namespace = chartRelease.Namespace
	chart.Status.Error = ""
	err = cr.Client.Status().Update(ctx, &chart)
	if err != nil {
		return fmt.Errorf("can't update status for `%s`: %v", chart.GetName(), err)
	}
	return nil
}

func (ec *ChartReconciler) saveError(ctx context.Context, origErr error, objectID string) {
	// 	name := strings.Split(objectID, "/")[1]
	// 	chart, err := ec.Client.Charts(namespaceToWatch).Get(ctx, name, metav1.GetOptions{})
	// 	if err != nil {
	// 		ec.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
	// 		return
	// 	}
	// 	if chart == nil {
	// 		return
	// 	}
	// 	chart.Status.Error = origErr.Error()
	// 	_, err = ec.Client.Charts(namespaceToWatch).UpdateStatus(ctx, chart, metav1.UpdateOptions{})
	// 	if err != nil {
	// 		ec.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
	// 	}
	// }

	// func (ec *ChartReconciler) uninstall(id string) error {
	// 	parts := strings.Split(id, "/")
	// 	namespace, releaseName := parts[0], parts[1]
	// 	if !ec.leaderElector.IsLeader() {
	// 		ec.L.Info("dry run, doesn't uninstall")
	// 		return nil
	// 	}
	// 	if err := ec.helm.UninstallRelease(releaseName, namespace); err != nil {
	// 		return fmt.Errorf("can't uninstall release `%s`: %v", releaseName, err)
	// 	}
	// return nil
}

func (ec *ExtensionsController) addRepo(repo k0sAPI.Repository) error {
	return ec.helm.AddRepository(repo)
}

const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: "kube-system"
spec:
  chartName: {{ .ChartName }}
  values: |
{{ .Values | nindent 4 }}
  version: {{ .Version }}
  namespace: {{ .TargetNS }}
`

// Init
func (h *ExtensionsController) Init() error {
	return nil
}

// Run
func (h *ExtensionsController) Run(ctx context.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", h.kubeConfig)
	if err != nil {
		return fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}

	mgr, err := manager.New(config, manager.Options{
		MetricsBindAddress: "0",
	})
	if err != nil {
		return fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	retry.Do(func() error {
		_, err := mgr.GetRESTMapper().RESTMapping(schema.GroupKind{
			Group: v1beta1.GroupVersion.Group,
			Kind:  "Chart",
		})
		if err != nil {
			h.L.Warn("Extensions CRD is not yet ready, waiting before starting ExtensionsController")
			return err
		}
		h.L.Info("Extensions CRD is ready, going nuts")
		return nil
	})
	// examples say to not use GetScheme in production, but it is unclear at the moment
	// which scheme should be in use
	if err := v1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("can't register Chart crd: %w", err)
	}
	if err := builder.
		ControllerManagedBy(mgr).
		For(&v1beta1.Chart{},
			builder.WithPredicates(predicate.And(
				predicate.GenerationChangedPredicate{},
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					return object.GetNamespace() == namespaceToWatch
				}),
			),
			),
		).
		Complete(&ChartReconciler{
			leaderElector: h.leaderElector, // TODO: drop in favor of controller-runtime lease manager
			helm:          h.helm,
		}); err != nil {
		return fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	spew.Dump("HELM: builder created")

	go mgr.Start(ctx)
	spew.Dump("HELM: Manager started")
	return nil
}

// Stop
func (h *ExtensionsController) Stop() error {
	return nil
}

// Healthy
func (h *ExtensionsController) Healthy() error {
	return nil
}
