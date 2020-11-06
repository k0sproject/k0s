package server

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	mkev1beta1 "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"time"
)

// Helm watch for Chart crd
type HelmAddons struct {
	client        clientset.ChartV1Beta1Interface
	ClusterConfig *mkev1beta1.ClusterConfig
	saver         manifestsSaver
	l             *logrus.Entry
}

// NewHelmAddons builds new HelmAddons
func NewHelmAddons(c *mkev1beta1.ClusterConfig, s manifestsSaver) *HelmAddons {
	return &HelmAddons{
		ClusterConfig: c,
		saver:         s,
		l:             logrus.WithFields(logrus.Fields{"component": "helmaddons"}),
	}
}

var chartResyncPeriod = time.Second * 30

func watchHelmCharts(client clientset.ChartV1Beta1Interface) (cache.Store, cache.Controller) {
	projectStore, projectController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return client.Charts("kube-system").List(context.Background())
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return client.Charts("kube-system").Watch(context.Background(), lo)
			},
		},
		&v1beta1.Chart{},
		chartResyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				spew.Dump("CREATE", obj)
			},

			UpdateFunc: func(oldObj, newObj interface{}) {
				old := oldObj.(*v1beta1.Chart)
				new := newObj.(*v1beta1.Chart)
				spew.Dump("UPDATE", old, new)
			},

			DeleteFunc: func(obj interface{}) {
				spew.Dump("DELETE", obj)
			},
		},
	)

	return projectStore, projectController
}

// Init
func (h HelmAddons) Run() error {
	if h.ClusterConfig.HelmAddons == nil {
		h.l.Info("No helm addons specified, do not run HelmAddons reconciler")
		return nil
	}
	client, err := clientset.NewForConfig(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return fmt.Errorf("can't create kubernetes typed client for helm charts: %v", err)
	}

	h.client = client

	if err := h.initHelm(); err != nil {
		return fmt.Errorf("can't init helm: %v", err)
	}

	h.l.Info("Successfully inited helm")
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
			h.l.WithError(err).Errorf("can't render helm addon crd template")
			return fmt.Errorf("can't create addon `%s`: %v", addon.ChartName, err)
		}
		if err := h.saver.Save("addon_crd_manifest_"+addon.Name+".yaml", buf.Bytes()); err != nil {
			return fmt.Errorf("can't save addon CRD manifest: %v", err)
		}
	}
	return nil
}

const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: mke-addon-chart-{{ .Name }}
  namespace: {{ .TargetNS }}
spec:
  chartName: {{ .ChartName }}
  values: |
  {{ .Values }}
  version: stable
status:
  status: created
`

func (h HelmAddons) addRepo(repository mkev1beta1.Repository) error {
	return nil
}

// Run
func (h HelmAddons) Init() error {
	return nil
}

// Stop
func (h HelmAddons) Stop() error {
	return nil
}

// Healthy
func (h HelmAddons) Healthy() error {
	return nil
}
