/*
Copyright 2022 k0s authors

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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"k8s.io/client-go/rest"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
)

const (
	namespace       = "k0s-system"
	pushGatewayName = "k0s-pushgateway"
)

// Metrics is the reconciler implementation for metrics server
type Metrics struct {
	log logrus.FieldLogger

	hostname   string
	K0sVars    *config.CfgVars
	saver      manifestsSaver
	restClient rest.Interface

	clusterConfig *v1beta1.ClusterConfig
	tickerDone    context.CancelFunc
	jobs          []*job
}

var _ manager.Component = (*Metrics)(nil)
var _ manager.Reconciler = (*Metrics)(nil)

// NewMetrics creates new Metrics reconciler
func NewMetrics(k0sVars *config.CfgVars, saver manifestsSaver, clientCF kubernetes.ClientFactoryInterface) (*Metrics, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	restClient, err := clientCF.GetRESTClient()
	if err != nil {
		return nil, fmt.Errorf("error getting REST client for metrics: %w", err)
	}

	return &Metrics{
		log: logrus.WithFields(logrus.Fields{"component": "metrics"}),

		hostname:   hostname,
		K0sVars:    k0sVars,
		saver:      saver,
		restClient: restClient,
	}, nil
}

// Init does nothing
func (m *Metrics) Init(_ context.Context) error {
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

	return nil
}

// Run runs the metric server reconciler
func (m *Metrics) Start(ctx context.Context) error {
	ctx, m.tickerDone = context.WithCancel(ctx)

	for _, j := range m.jobs {
		go j.Run(ctx)
	}

	return nil
}

// Stop stops the reconciler
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
		err = m.saver.Save("pushgateway.yaml", output.Bytes())
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
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
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
