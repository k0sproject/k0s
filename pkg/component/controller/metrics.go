// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
)

const (
	namespace       = "k0s-system"
	pushGatewayName = "k0s-pushgateway"
)

// Metrics is the reconciler implementation for metrics server
type Metrics struct {
	log logrus.FieldLogger

	hostname    string
	K0sVars     *config.CfgVars
	restClient  rest.Interface
	storageType v1beta1.StorageType

	clusterConfig *v1beta1.ClusterConfig
	tickerDone    context.CancelFunc
	jobs          []*job
}

var _ manager.Component = (*Metrics)(nil)
var _ manager.Reconciler = (*Metrics)(nil)

// NewMetrics creates new Metrics reconciler
func NewMetrics(k0sVars *config.CfgVars, clientCF kubeutil.ClientFactoryInterface, storageType v1beta1.StorageType) (*Metrics, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	var restClient rest.Interface
	if client, err := clientCF.GetDiscoveryClient(); err != nil {
		return nil, fmt.Errorf("error getting REST client for metrics: %w", err)
	} else {
		restClient = client.RESTClient()
	}
	if restClient == nil {
		return nil, errors.New("no REST client for metrics")
	}

	return &Metrics{
		log:         logrus.WithFields(logrus.Fields{"component": "metrics"}),
		storageType: storageType,
		hostname:    hostname,
		K0sVars:     k0sVars,
		restClient:  restClient,
	}, nil
}

// Init implements [manager.Component].
func (m *Metrics) Init(context.Context) error {
	if err := dir.Init(filepath.Join(m.K0sVars.ManifestsDir, "metrics"), constant.ManifestsDirMode); err != nil {
		return err
	}

	var j *job
	j, err := m.newJob("kube-scheduler", "https://localhost:10259/metrics")
	if err != nil {
		return err
	}
	m.jobs = append(m.jobs, j)

	j, err = m.newJob("kube-controller-manager", "https://localhost:10257/metrics")
	if err != nil {
		return err
	}
	m.jobs = append(m.jobs, j)

	if m.storageType == v1beta1.EtcdStorageType {
		etcdJob, err := m.newEtcdJob()
		if err != nil {
			return err
		}
		m.jobs = append(m.jobs, etcdJob)
	}

	if m.storageType == v1beta1.KineStorageType {
		kineJob, err := m.newKineJob()
		if err != nil {
			return err
		}
		m.jobs = append(m.jobs, kineJob)
	}

	return nil
}

// Start implements [manager.Component].
// Starts the metric server reconciler.
func (m *Metrics) Start(ctx context.Context) error {
	ctx, m.tickerDone = context.WithCancel(ctx)

	for _, j := range m.jobs {
		go j.Run(ctx)
	}

	return nil
}

// Stop implements [manager.Component].
// Stops the metric server reconciler.
func (m *Metrics) Stop() error {
	if m.tickerDone != nil {
		m.tickerDone()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (m *Metrics) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	m.log.Debug("reconcile method called for: Metrics")

	if m.clusterConfig == nil || clusterConfig.Spec.Images.PushGateway.URI() != m.clusterConfig.Spec.Images.PushGateway.URI() {
		tw := templatewriter.TemplateWriter{
			Name:     "pushgateway-with-ttl",
			Template: pushGatewayTemplate,
			Data: map[string]string{
				"Namespace": namespace,
				"Name":      pushGatewayName,
				"Image":     clusterConfig.Spec.Images.PushGateway.URI(),
			},
		}
		output := bytes.NewBuffer([]byte{})
		err := tw.WriteToBuffer(output)
		if err != nil {
			return err
		}
		err = file.AtomicWithTarget(filepath.Join(m.K0sVars.ManifestsDir, "metrics", "pushgateway.yaml")).
			WithPermissions(constant.CertMode).
			Write(output.Bytes())
		if err != nil {
			return err
		}
	}

	// We just store the last known config
	for _, j := range m.jobs {
		j.clusterConfig = clusterConfig
	}
	m.clusterConfig = clusterConfig
	return nil
}

type job struct {
	log logrus.FieldLogger

	scrapeURL     string
	name          string
	hostname      string
	clusterConfig *v1beta1.ClusterConfig
	scrapeClient  *http.Client
	restClient    rest.Interface
}

func (m *Metrics) newEtcdJob() (*job, error) {
	certFile := path.Join(m.K0sVars.CertRootDir, "apiserver-etcd-client.crt")
	keyFile := path.Join(m.K0sVars.CertRootDir, "apiserver-etcd-client.key")

	httpClient, err := getClient(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &job{
		log:          m.log.WithField("metrics_job", "etcd"),
		scrapeURL:    "https://localhost:2379/metrics",
		name:         "etcd",
		hostname:     m.hostname,
		scrapeClient: httpClient,
		restClient:   m.restClient,
	}, nil
}

func (m *Metrics) newKineJob() (*job, error) {
	httpClient, err := getClient("", "")
	if err != nil {
		return nil, err
	}

	return &job{
		log:          m.log.WithField("metrics_job", "kine"),
		scrapeURL:    "http://localhost:2380/metrics",
		name:         "kine",
		hostname:     m.hostname,
		scrapeClient: httpClient,
		restClient:   m.restClient,
	}, nil
}

func (m *Metrics) newJob(name, scrapeURL string) (*job, error) {
	certFile := path.Join(m.K0sVars.CertRootDir, "admin.crt")
	keyFile := path.Join(m.K0sVars.CertRootDir, "admin.key")

	httpClient, err := getClient(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &job{
		log:          m.log.WithField("metrics_job", name),
		scrapeURL:    scrapeURL,
		name:         name,
		hostname:     m.hostname,
		scrapeClient: httpClient,
		restClient:   m.restClient,
	}, nil
}

func (j *job) Run(ctx context.Context) {
	j.log.Debugf("Running %s job", j.name)

	t := time.NewTicker(time.Second * 30)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if j.clusterConfig == nil {
				continue
			}

			err := j.collectAndPush(ctx)
			if err != nil {
				j.log.Error(err)
			}
		}
	}
}
func (j *job) pushURL() string {
	pushAddress := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s:http/proxy", namespace, pushGatewayName)
	return fmt.Sprintf("%s/metrics/job/%s/instance/%s", pushAddress, j.name, j.hostname)
}

func (j *job) collectAndPush(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.scrapeURL, nil)
	if err != nil {
		return fmt.Errorf("error creating GET request for %s: %w", j.scrapeURL, err)
	}

	resp, err := j.scrapeClient.Do(req)
	if err != nil {
		return fmt.Errorf("error collecting metrics from %s: %w", j.scrapeURL, err)
	}
	defer resp.Body.Close()

	res := j.restClient.Post().AbsPath(j.pushURL()).Body(resp.Body).Do(ctx)
	if res.Error() != nil {
		return fmt.Errorf("error sending POST request for job %s: %w", j.name, res.Error())
	}
	return nil
}

func getClient(certFile, keyFile string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Minute
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport.TLSClientConfig = tlsConfig

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Minute,
	}, nil
}

const pushGatewayTemplate = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    prometheus.io/port: "9091"
    prometheus.io/scrape: "true"
  labels:
    component: "pushgateway"
    app: k0s-observability
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  ports:
    - name: http
      port: 9091
      protocol: TCP
      targetPort: 9091
  selector:
    component: "pushgateway"
    app: k0s-observability
  type: "ClusterIP"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    component: "pushgateway"
    app: k0s-observability
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  selector:
    matchLabels:
      component: "pushgateway"
      app: k0s-observability
  replicas: 1
  template:
    metadata:
      labels:
        component: "pushgateway"
        app: k0s-observability
    spec:
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
      containers:
        - name: prometheus-pushgateway
          image: {{ .Image }}
          imagePullPolicy: "IfNotPresent"
          args: 
          - --metric.timetolive=120s
          ports:
            - containerPort: 9091
          livenessProbe:
            httpGet:
              path: /-/healthy
              port: 9091
            initialDelaySeconds: 10
            timeoutSeconds: 10
          readinessProbe:
            httpGet:
              path: /-/ready
              port: 9091
            initialDelaySeconds: 10
            timeoutSeconds: 10
          resources:
            {}
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
`
