package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/pkg/util"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Helm watch for Chart crd
type HelmAddons struct {
	Client        clientset.ChartV1Beta1Interface
	ClusterConfig *k0sv1beta1.ClusterConfig
	saver         manifestsSaver
	L             *logrus.Entry
	stopCh        chan struct{}
	informer      cache.SharedIndexInformer
	helm          *helm.Commands
	kubeConfig    string

	dryRun     bool
	dryRunLock sync.Mutex
}

// NewHelmAddons builds new HelmAddons
func NewHelmAddons(c *k0sv1beta1.ClusterConfig, s manifestsSaver, k0sVars constant.CfgVars) *HelmAddons {
	return &HelmAddons{
		ClusterConfig: c,
		saver:         s,
		L:             logrus.WithFields(logrus.Fields{"component": "helmaddons"}),
		stopCh:        make(chan struct{}),
		helm:          helm.NewCommands(k0sVars),
		kubeConfig:    k0sVars.AdminKubeconfigConfigPath,
	}
}

const (
	operationAdd    = "add"
	operationUpdate = "update"
	operationDelete = "delete"

	namespaceToWatch = "kube-system"
)

// Init
func (h *HelmAddons) Run() error {
	h.L.Info("run begin")
	if h.ClusterConfig.Extensions == nil || h.ClusterConfig.Extensions.Helm == nil {
		h.L.Info("No helm addons specified, do not run HelmAddons reconciler")
		return nil
	}
	client, err := clientset.NewForConfig(h.kubeConfig)
	if err != nil {
		return fmt.Errorf("can't create kubernetes typed Client for helm charts: %v", err)
	}

	h.Client = client

	if err := h.initHelm(); err != nil {
		return fmt.Errorf("can't init helm: %v", err)
	}

	h.L.Info("Successfully inited helm")
	if !cache.WaitForCacheSync(h.stopCh) {
		panic("Can't sync cache")
	}
	h.L.Info("Successfully synced controller cache")

	if err := h.leaseLoop(); err != nil {
		return fmt.Errorf("can't init lease pool: %v", err)
	}
	go h.CrdControlLoop()
	return nil
}

func (h *HelmAddons) leaseLoop() error {
	client, err := kubeutil.Client(h.kubeConfig)
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %v", err)
	}
	leasePool, err := leaderelection.NewLeasePool(client, "k0s-helm-addons", leaderelection.WithLogger(h.L))

	if err != nil {
		return err
	}
	events, _, err := leasePool.Watch()

	if err != nil {
		return err
	}

	go func() {

		for {
			select {
			case <-events.AcquiredLease:
				h.L.Info("acquired leader lease")
				h.dryRunLock.Lock()
				h.dryRun = false
				h.dryRunLock.Unlock()

			case <-events.LostLease:
				h.L.Info("lost leader lease")
				h.dryRunLock.Lock()
				h.dryRun = true
				h.dryRunLock.Unlock()
			}
		}
	}()
	return nil
}

func (h *HelmAddons) initHelm() error {
	for _, repo := range h.ClusterConfig.Extensions.Helm.Repositories {
		if err := h.addRepo(repo); err != nil {
			return fmt.Errorf("can't init repository `%s`: %v", repo.URL, err)
		}
	}

	for _, addon := range h.ClusterConfig.Extensions.Helm.Charts {
		tw := util.TemplateWriter{
			Name:     "addon_crd_manifest",
			Template: chartCrdTemplate,
			Data:     addon,
		}
		buf := bytes.NewBuffer([]byte{})
		if err := tw.WriteToBuffer(buf); err != nil {
			h.L.WithError(err).Errorf("can't render helm addon crd template")
			return fmt.Errorf("can't create addon `%s`: %v", addon.ChartName, err)
		}
		if err := h.saver.Save("addon_crd_manifest_"+addon.Name+".yaml", buf.Bytes()); err != nil {
			return fmt.Errorf("can't save addon CRD manifest: %v", err)
		}
	}
	return nil
}

type queueJob struct {
	key       string
	operation string
}

func (h *HelmAddons) CrdControlLoop() {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	defer queue.ShutDown()
	h.informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return h.Client.Charts(namespaceToWatch).List(context.Background())
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return h.Client.Charts(namespaceToWatch).Watch(context.Background(), lo)
			},
		},
		&v1beta1.Chart{},
		0,
		cache.Indexers{},
	)
	h.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				h.L.WithError(err).Warning("can't build cache key for queue object")
				return
			}
			queue.Add(queueJob{key: key, operation: operationAdd})
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			old := oldObj.(*v1beta1.Chart)
			new := newObj.(*v1beta1.Chart)

			if old.Generation == new.Generation {
				return
			}

			if old.ObjectMeta.ResourceVersion == new.ObjectMeta.ResourceVersion {
				return
			}

			key, err := cache.MetaNamespaceKeyFunc(new)
			if err != nil {
				h.L.WithError(err).Warning("can't build cache key for queue object")
				return
			}
			queue.Add(queueJob{key: key, operation: operationUpdate})
		},

		DeleteFunc: func(obj interface{}) {
			chart := obj.(*v1beta1.Chart)
			queue.Add(queueJob{key: chart.Status.Namespace + "/" + chart.Status.ReleaseName, operation: operationDelete})
		},
	})
	go h.informer.Run(h.stopCh)
	wait.Until(func() {
		for {
			h.processMessage(queue)
		}
	}, time.Second, h.stopCh)
}

const maxRetries = 5

func (h *HelmAddons) processMessage(q workqueue.RateLimitingInterface) {
	jobI, quit := q.Get()
	job := jobI.(queueJob)

	if quit {
		return
	}

	defer q.Done(job)

	var err error
	switch job.operation {
	case operationDelete:
		err = h.uninstall(job.key)
	case operationAdd, operationUpdate:
		err = h.reconcile(job.key)
	}

	if err != nil {
		if q.NumRequeues(job) < maxRetries {
			h.L.WithError(err).Errorf("Error processing %s (will retry)", job.key)
			q.AddRateLimited(job)
			return
		}
		h.saveError(err, job.key)
		h.L.WithError(err).Errorf("Error processing %s (giving up)", job.key)

	}

	q.Forget(job)

}

func (h *HelmAddons) saveError(origErr error, objectID string) {
	name := strings.Split(objectID, "/")[1]
	chart, err := h.Client.Charts(namespaceToWatch).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		h.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
		return
	}
	if chart == nil {
		return
	}
	chart.Status.Error = origErr.Error()
	_, err = h.Client.Charts(namespaceToWatch).UpdateStatus(context.Background(), chart, metav1.UpdateOptions{})
	if err != nil {
		h.L.Errorf("can't save error to the chart CRD status `%s`: %v", objectID, err)
	}
}

func (h *HelmAddons) uninstall(id string) error {
	parts := strings.Split(id, "/")
	namespace, releaseName := parts[0], parts[1]
	if h.dryRun {
		h.L.Info("dry run, doesn't uninstall")
		return nil
	}
	if err := h.helm.UninstallRelease(releaseName, namespace); err != nil {
		return fmt.Errorf("can't uninstall release `%s`: %v", releaseName, err)
	}
	return nil
}

func (h *HelmAddons) reconcile(objectID string) error {

	if h.dryRun {
		h.L.Info("dry run, doesn't reconcile")
		return nil
	}
	name := strings.Split(objectID, "/")[1]
	chart, err := h.Client.Charts(namespaceToWatch).Get(context.Background(), name, metav1.GetOptions{})

	if err != nil {
		return fmt.Errorf("can't reconcile chart `%s`: %v", objectID, err)
	}
	var release *release.Release
	if chart.Status.ReleaseName == "" {
		// new release
		release, err = h.helm.InstallChart(chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Spec.Namespace,
			chart.Spec.YamlValues())
		if err != nil {
			return fmt.Errorf("can't reconcile installation for `%s`: %v", objectID, err)
		}
	} else {
		// update
		release, err = h.helm.UpgradeChart(chart.Spec.ChartName,
			chart.Status.Version,
			chart.Status.ReleaseName,
			chart.Status.Namespace,
			chart.Spec.YamlValues(),
		)
		if err != nil {
			return fmt.Errorf("can't reconcile upgrade for `%s`: %v", objectID, err)
		}
	}

	chart.Status.ReleaseName = release.Name
	chart.Status.Version = release.Chart.Metadata.Version
	chart.Status.AppVersion = release.Chart.AppVersion()
	chart.Status.Updated = time.Now().String()
	chart.Status.Revision = int64(release.Version)
	chart.Status.Namespace = release.Namespace
	chart.Status.Error = ""
	_, err = h.Client.Charts(namespaceToWatch).UpdateStatus(context.Background(), chart, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update status for `%s`: %v", objectID, err)
	}
	return nil
}

func (h *HelmAddons) addRepo(repo k0sv1beta1.Repository) error {
	return h.helm.AddRepository(repo)
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

// Run
func (h *HelmAddons) Init() error {
	return nil
}

// Stop
func (h *HelmAddons) Stop() error {
	close(h.stopCh)
	return nil
}

// Healthy
func (h *HelmAddons) Healthy() error {
	return nil
}
