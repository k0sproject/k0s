package server

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	mkev1beta1 "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/helm"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"strings"
	"time"
)

// Helm watch for Chart crd
type HelmAddons struct {
	Client        clientset.ChartV1Beta1Interface
	ClusterConfig *mkev1beta1.ClusterConfig
	saver         manifestsSaver
	L             *logrus.Entry
	stopCh        chan struct{}
	informer      cache.SharedIndexInformer
	helm          *helm.Commands
	queue         workqueue.RateLimitingInterface
}

// NewHelmAddons builds new HelmAddons
func NewHelmAddons(c *mkev1beta1.ClusterConfig, s manifestsSaver) *HelmAddons {
	return &HelmAddons{
		ClusterConfig: c,
		saver:         s,
		L:             logrus.WithFields(logrus.Fields{"component": "helmaddons"}),
		stopCh:        make(chan struct{}),
		helm:          helm.NewCommands(),
	}
}

const (
	operationAdd    = "add"
	operationUpdate = "update"
	operationDelete = "delete"

	namespaceToWatch = "kube-system"
)

var chartResyncPeriod = time.Second * 30

// Init
func (h HelmAddons) Run() error {
	h.L.Info("run begin")
	if h.ClusterConfig.HelmAddons == nil {
		h.L.Info("No helm addons specified, do not run HelmAddons reconciler")
		return nil
	}
	client, err := clientset.NewForConfig(constant.AdminKubeconfigConfigPath)
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
	go h.CrdControlLoop()
	return nil
}

func (h HelmAddons) initHelm() error {
	spew.Dump(h.ClusterConfig.HelmAddons)
	for _, repo := range h.ClusterConfig.HelmAddons.Repositories {
		if err := h.addRepo(repo); err != nil {
			return fmt.Errorf("can't init repository `%s`: %v", repo.URL, err)
		}
	}

	for _, addon := range h.ClusterConfig.HelmAddons.Addons {
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

func (h HelmAddons) CrdControlLoop() {
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
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				h.L.WithError(err).Warning("can't build cache key for queue object")
				return
			}
			queue.Add(queueJob{key: key, operation: operationDelete})
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

func (h HelmAddons) processMessage(q workqueue.RateLimitingInterface) {
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
		h.L.WithError(err).Errorf("Error processing %s (giving up)", job.key)
	}

	q.Forget(job)

}

func (h HelmAddons) uninstall(objectId string) error {
	return nil
}

func (h HelmAddons) reconcile(objectId string) error {
	// - if no release name, install and update with metadata
	// - if release name, update and update status
	name := strings.Split(objectId, "/")[1]
	chart, err := h.Client.Charts(namespaceToWatch).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't reconcile chart `%s`: %v", objectId, err)
	}
	var releaseName string
	if chart.Status.ReleaseName == "" {
		// new release
		releaseName, err = h.helm.InstallChart(chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Spec.Namespace,
			chart.Spec.YamlValues())
		if err != nil {
			return fmt.Errorf("can't reconcile installation for `%s`: %v", objectId, err)
		}
	} else {
		// update
		// Probably, it could be better to compare values here and decide do we want do actual update, or just leave it
		// to the helm machinery?
		releaseName, err = h.helm.UpgradeChart(chart.Spec.ChartName,
			chart.Spec.Version,
			chart.Status.ReleaseName,
			chart.Spec.Namespace,
			chart.Spec.YamlValues(),
		)
		if err != nil {
			return fmt.Errorf("can't reconcile upgrade for `%s`: %v", objectId, err)
		}
	}

	chart.Status.ReleaseName = releaseName
	chart.Status.Updated = time.Now().String()
	_, err = h.Client.Charts(namespaceToWatch).UpdateStatus(context.Background(), chart, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update status for `%s`: %v", objectId, err)
	}
	return nil
}

func (h HelmAddons) addRepo(repo mkev1beta1.Repository) error {
	return h.helm.AddRepository(repo)
}

const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: mke-addon-chart-{{ .Name }}
  namespace: "kube-system"
spec:
  chartName: {{ .ChartName }}
  values: |
{{ .Values | nindent 4 }}
  version: {{ .Version }}
  namespace: {{ .TargetNS }}
`

// Run
func (h HelmAddons) Init() error {
	return nil
}

// Stop
func (h HelmAddons) Stop() error {
	close(h.stopCh)
	return nil
}

// Healthy
func (h HelmAddons) Healthy() error {
	return nil
}
