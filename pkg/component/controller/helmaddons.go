package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/v1beta1"
	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// Helm watch for Chart crd
type ExtensionsController struct {
	Client clientset.ChartV1Beta1Interface

	saver             manifestsSaver
	L                 *logrus.Entry
	stopCh            chan struct{}
	informer          cache.SharedIndexInformer
	helm              *helm.Commands
	kubeConfig        string
	kubeClientFactory kubeutil.ClientFactoryInterface
	leaderElector     LeaderElector
	crdSync           sync.Once
}

var _ component.Component = &ExtensionsController{}
var _ component.ReconcilerComponent = &ExtensionsController{}

// NewExtensionsController builds new HelmAddons
func NewExtensionsController(s manifestsSaver, k0sVars constant.CfgVars, kubeClientFactory kubeutil.ClientFactoryInterface, leaderElector LeaderElector) *ExtensionsController {
	return &ExtensionsController{
		saver:             s,
		L:                 logrus.WithFields(logrus.Fields{"component": "helmaddons"}),
		stopCh:            make(chan struct{}),
		helm:              helm.NewCommands(k0sVars),
		kubeConfig:        k0sVars.AdminKubeConfigPath,
		kubeClientFactory: kubeClientFactory,
		leaderElector:     leaderElector,
	}
}

const (
	operationAdd    = "add"
	operationUpdate = "update"
	operationDelete = "delete"

	namespaceToWatch = "kube-system"
)

// Run runs the extensions controller
func (ec *ExtensionsController) Reconcile(ctx context.Context, clusterConfig *k0sAPI.ClusterConfig) error {
	ec.L.Info("Extensions reconcilation started")
	defer ec.L.Info("Extensions reconcilation finished")

	// TODO Can we use the shared kube client factory to create the clientset for helm CRDs?
	client, err := clientset.NewForConfig(ec.kubeConfig)
	if err != nil {
		return fmt.Errorf("can't create kubernetes typed Client for helm charts: %v", err)
	}

	ec.Client = client

	if err := ec.reconcileHelmExtensions(clusterConfig.Spec.Extensions.Helm); err != nil {
		return fmt.Errorf("can't reconcile helm based extensions: %v", err)
	}

	ec.L.Info("Successfully inited helm")
	if !cache.WaitForCacheSync(ec.stopCh) {
		panic("Can't sync cache")
	}
	ec.L.Info("Successfully synced controller cache")

	// temporary fix for routines leak introduced by dynamic configuration
	ec.crdSync.Do(func() {
		go ec.CrdControlLoop(ctx)
	})
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

type queueJob struct {
	key       string
	operation string
}

func (ec *ExtensionsController) CrdControlLoop(ctx context.Context) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	defer queue.ShutDown()
	ec.informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return ec.Client.Charts(namespaceToWatch).List(ctx)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return ec.Client.Charts(namespaceToWatch).Watch(ctx, lo)
			},
		},
		&v1beta1.Chart{},
		0,
		cache.Indexers{},
	)
	ec.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				ec.L.WithError(err).Warning("can't build cache key for queue object")
				return
			}
			queue.Add(queueJob{key: key, operation: operationAdd})
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldChart := oldObj.(*v1beta1.Chart)
			newChart := newObj.(*v1beta1.Chart)

			if oldChart.Generation == newChart.Generation {
				return
			}

			if oldChart.ObjectMeta.ResourceVersion == newChart.ObjectMeta.ResourceVersion {
				return
			}

			key, err := cache.MetaNamespaceKeyFunc(newChart)
			if err != nil {
				ec.L.WithError(err).Warning("can't build cache key for queue object")
				return
			}
			queue.Add(queueJob{key: key, operation: operationUpdate})
		},

		DeleteFunc: func(obj interface{}) {
			chart := obj.(*v1beta1.Chart)
			queue.Add(queueJob{key: chart.Status.Namespace + "/" + chart.Status.ReleaseName, operation: operationDelete})
		},
	})
	go ec.informer.Run(ec.stopCh)
	wait.Until(func() {
		for {
			ec.processMessage(ctx, queue)
		}
	}, time.Second, ec.stopCh)
}

const maxRetries = 5

func (ec *ExtensionsController) processMessage(ctx context.Context, q workqueue.RateLimitingInterface) {
	jobI, quit := q.Get()
	job := jobI.(queueJob)

	if quit {
		return
	}

	defer q.Done(job)

	var err error
	switch job.operation {
	case operationDelete:
		err = ec.uninstall(job.key)
	case operationAdd, operationUpdate:
		err = ec.reconcile(ctx, job.key)
	}

	if err != nil {
		if q.NumRequeues(job) < maxRetries {
			ec.L.WithError(err).Errorf("Error processing %s (will retry)", job.key)
			q.AddRateLimited(job)
			return
		}
		ec.saveError(ctx, err, job.key)
		ec.L.WithError(err).Errorf("Error processing %s (giving up)", job.key)

	}

	q.Forget(job)
}

func (ec *ExtensionsController) saveError(ctx context.Context, origErr error, objectID string) {
	name := strings.Split(objectID, "/")[1]
	chart, err := ec.Client.Charts(namespaceToWatch).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		ec.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
		return
	}
	if chart == nil {
		return
	}
	chart.Status.Error = origErr.Error()
	_, err = ec.Client.Charts(namespaceToWatch).UpdateStatus(ctx, chart, metav1.UpdateOptions{})
	if err != nil {
		ec.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
	}
}

func (ec *ExtensionsController) uninstall(id string) error {
	parts := strings.Split(id, "/")
	namespace, releaseName := parts[0], parts[1]
	if !ec.leaderElector.IsLeader() {
		ec.L.Info("dry run, doesn't uninstall")
		return nil
	}
	if err := ec.helm.UninstallRelease(releaseName, namespace); err != nil {
		return fmt.Errorf("can't uninstall release `%s`: %v", releaseName, err)
	}
	return nil
}

func (ec *ExtensionsController) reconcile(ctx context.Context, objectID string) error {
	if !ec.leaderElector.IsLeader() {
		ec.L.Info("dry run, doesn't reconcile")
		return nil
	}
	name := strings.Split(objectID, "/")[1]
	chart, err := ec.Client.Charts(namespaceToWatch).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't reconcile chart `%s`: %v", objectID, err)
	}
	var chartRelease *release.Release
	if chart.Status.ReleaseName == "" {
		// new chartRelease
		chartRelease, err = ec.helm.InstallChart(chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Spec.Namespace,
			chart.Spec.YamlValues())
		if err != nil {
			return fmt.Errorf("can't reconcile installation for `%s`: %v", objectID, err)
		}
	} else {
		// update
		chartRelease, err = ec.helm.UpgradeChart(chart.Spec.ChartName,
			chart.Status.Version,
			chart.Status.ReleaseName,
			chart.Status.Namespace,
			chart.Spec.YamlValues(),
		)
		if err != nil {
			return fmt.Errorf("can't reconcile upgrade for `%s`: %v", objectID, err)
		}
	}

	chart.Status.ReleaseName = chartRelease.Name
	chart.Status.Version = chartRelease.Chart.Metadata.Version
	chart.Status.AppVersion = chartRelease.Chart.AppVersion()
	chart.Status.Updated = time.Now().String()
	chart.Status.Revision = int64(chartRelease.Version)
	chart.Status.Namespace = chartRelease.Namespace
	chart.Status.Error = ""
	_, err = ec.Client.Charts(namespaceToWatch).UpdateStatus(ctx, chart, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update status for `%s`: %v", objectID, err)
	}
	return nil
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
func (h *ExtensionsController) Run(_ context.Context) error {
	return nil
}

// Stop
func (h *ExtensionsController) Stop() error {
	close(h.stopCh)
	return nil
}

// Healthy
func (h *ExtensionsController) Healthy() error {
	return nil
}
