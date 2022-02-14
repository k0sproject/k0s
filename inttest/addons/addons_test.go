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

package addons

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/v1beta1"
)

type AddonsSuite struct {
	common.FootlooseSuite
}

func (as *AddonsSuite) TestHelmBasedAddons() {
	addonName := "test-addon"
	as.PutFile(as.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithAddon, addonName))

	as.Require().NoError(as.InitController(0, "--config=/tmp/k0s.yaml"))
	as.waitForPrometheusRelease(addonName, 1)

	values := map[string]interface{}{
		"server": map[string]interface{}{
			"env": []interface{}{
				map[string]interface{}{
					"name":  "FOO",
					"value": "foobar",
				},
			},
		},
	}
	as.doPrometheusUpdate(addonName, values)
	chart := as.waitForPrometheusRelease(addonName, 2)
	as.Require().NoError(as.waitForPrometheusServerEnvs(chart.Status.ReleaseName))
	as.doPrometheusDelete(chart)
}

func (as *AddonsSuite) doPrometheusDelete(chart *v1beta1.Chart) {
	as.T().Logf("Deleting chart %s/%s", chart.Namespace, chart.Name)
	ssh, err := as.SSH(as.ControllerNode(0))
	as.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("rm /var/lib/k0s/manifests/helm/addon_crd_manifest_test-addon.yaml")
	as.Require().NoError(err)

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	k8sclient, err := k8s.NewForConfig(cfg)
	as.Require().NoError(err)
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		as.T().Logf("Expecting have no secrets left for release %s/%s", chart.Namespace, chart.Name)
		items, err := k8sclient.CoreV1().Secrets("default").List(context.Background(), v1.ListOptions{})
		if err != nil {
			as.T().Logf("listing secrets error %s", err.Error())
			return false, nil
		}
		if len(items.Items) > 1 {
			return false, nil
		}
		as.T().Log("Release uninstalled successfully")
		return true, nil
	}))
}

func (as *AddonsSuite) waitForPrometheusRelease(addonName string, rev int64) *v1beta1.Chart {
	as.T().Logf("waiting to see prometheus release ready in kube API, generation %d", rev)

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	v1beta1.AddToScheme(scheme.Scheme)
	chartClient, err := client.New(cfg, client.Options{
		Scheme: scheme.Scheme,
	})
	as.Require().NoError(err)
	var chart v1beta1.Chart
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		err = chartClient.Get(context.Background(), client.ObjectKey{
			Namespace: "kube-system",
			Name:      fmt.Sprintf("k0s-addon-chart-%s", addonName),
		}, &chart)
		if err != nil {
			as.T().Log("Error while quering for chart", err)
			return false, nil
		}
		if chart.Status.ReleaseName == "" {
			return false, nil
		}
		if chart.Generation != rev {
			return false, nil
		}
		if chart.Status.Revision != rev {
			return false, nil
		}

		as.Require().Equal("default", chart.Status.Namespace)
		as.Require().Equal("2.26.0", chart.Status.AppVersion)
		as.Require().Equal("default", chart.Status.Namespace)
		as.Require().NotEmpty(chart.Status.ReleaseName)
		as.Require().Empty(chart.Status.Error)
		as.Require().Equal(rev, chart.Status.Revision)
		as.T().Logf("found test addon release: %s\n", chart.Name)
		as.Require().Equal(rev, chart.Generation)
		return true, nil
	}))
	return &chart
}

func (as *AddonsSuite) waitForPrometheusServerEnvs(releaseName string) error {
	as.T().Logf("waiting to see prometheus release to have envs set from values yaml")
	kc, err := as.KubeClient(as.ControllerNode(0))
	if err != nil {
		return err
	}

	return wait.PollImmediate(time.Second, 2*time.Minute, func() (done bool, err error) {
		serverDeployment := fmt.Sprintf("%s-prometheus-server", releaseName)
		d, err := kc.AppsV1().Deployments("default").Get(context.TODO(), serverDeployment, v1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name == "prometheus-server" {
				for _, e := range c.Env {
					if e.Name == "FOO" && e.Value == "foobar" {
						return true, nil
					}
				}
			} else {
				continue
			}
		}
		return false, nil
	})
}

func (as *AddonsSuite) doPrometheusUpdate(addonName string, values map[string]interface{}) {
	path := fmt.Sprintf("/var/lib/k0s/manifests/helm/addon_crd_manifest_%s.yaml", addonName)
	valuesBytes, err := yaml.Marshal(values)
	as.Require().NoError(err)
	tw := templatewriter.TemplateWriter{
		Name:     "testChartUpdate",
		Template: chartCrdTemplate,
		Data: struct {
			Name      string
			ChartName string
			Values    string
			Version   string
			TargetNS  string
		}{
			Name:      "test-addon",
			ChartName: "prometheus-community/prometheus",
			Values:    string(valuesBytes),
			Version:   "14.6.0",
			TargetNS:  "default",
		},
	}
	buf := bytes.NewBuffer([]byte{})
	as.Require().NoError(tw.WriteToBuffer(buf))

	as.PutFile(as.ControllerNode(0), path, buf.String())
}

func TestAddonsSuite(t *testing.T) {

	s := AddonsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}

	suite.Run(t, &s)

}

const k0sConfigWithAddon = `
spec:
    extensions:
        helm:
          repositories:
          - name: stable
            url: https://charts.helm.sh/stable
          - name: prometheus-community
            url: https://prometheus-community.github.io/helm-charts
          charts:
          - name: %s
            chartname: prometheus-community/prometheus
            version: "14.6.0"
            values: ""
            namespace: default
`

// TODO: this actually duplicates logic from the controller code
// better to somehow handle it by programmatic api
const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: "kube-system"
  finalizers:
    - helm.k0sproject.io/uninstall-helm-release 
spec:
  chartName: {{ .ChartName }}
  releaseName: {{ .Name }}
  values: |
{{ .Values | nindent 4 }}
  version: {{ .Version }}
  namespace: {{ .TargetNS }}
`
