/*
Copyright 2020 Mirantis, Inc.

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
	"github.com/Mirantis/mke/inttest/common"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
	"time"
)

type AddonsSuite struct {
	common.FootlooseSuite
}

func (as *AddonsSuite) TestHelmBasedAddons() {
	addonName := "test-addon"
	as.prepareConfigWithAddons(addonName)

	as.Require().NoError(as.InitMainController("/tmp/mke.yaml"))
	as.waitForPrometheusRelease(addonName, 1)

	as.doPrometheusUpdate(addonName, map[string]interface{}{"key": "value"})
	chartName := as.waitForPrometheusRelease(addonName, 2)
	as.doPrometheusDelete(chartName)
}

func (as *AddonsSuite) doPrometheusDelete(chartName string) {
	cfg := as.getKubeConfig("controller0")
	chartClient, err := clientset.New(cfg)
	as.Require().NoError(err)
	as.Require().NoError(chartClient.Charts("kube-system").Delete(context.Background(), chartName, v1.DeleteOptions{}))
	client, err := k8s.NewForConfig(cfg)
	as.Require().NoError(err)
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		items, err := client.CoreV1().Secrets("default").List(context.Background(), v1.ListOptions{})
		if err != nil {
			as.T().Logf("listing secrets error %s", err.Error())
			return false, nil
		}
		if len(items.Items) > 1 {
			return false, nil
		}
		return true, nil
	}))
}

func (as *AddonsSuite) waitForPrometheusRelease(addonName string, rev int64) string {
	as.T().Logf("waiting to see prometheus release ready in kube API, generation %d", rev)
	cfg := as.getKubeConfig("controller0")
	chartClient, err := clientset.New(cfg)
	as.Require().NoError(err)
	var chartName string
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		charts, err := chartClient.Charts("kube-system").List(context.Background())
		if err != nil {
			return false, nil
		}
		if len(charts.Items) == 0 {
			return false, nil
		}
		found := false
		var testAddonItem v1beta1.Chart
		for _, item := range charts.Items {
			if item.Name == fmt.Sprintf("mke-addon-chart-%s", addonName) {
				if item.Status.ReleaseName == "" {
					return false, nil
				}
				if item.Generation != rev {
					return false, nil
				}
				if item.Status.Revision != rev {
					return false, nil
				}
				found = true
				testAddonItem = item
				break
			}
		}
		as.Require().True(found)
		as.Require().Equal("default", testAddonItem.Status.Namespace)
		as.Require().Equal("2.21.0", testAddonItem.Status.AppVersion)
		as.Require().Equal("default", testAddonItem.Status.Namespace)
		as.Require().NotEmpty(testAddonItem.Status.ReleaseName)
		as.Require().Empty(testAddonItem.Status.Error)
		as.Require().Equal(rev, testAddonItem.Status.Revision)
		as.T().Logf("found test addon release: %s\n", testAddonItem.Name)
		as.Require().Equal(rev, testAddonItem.Generation)
		chartName = testAddonItem.Name
		return true, nil
	}))
	return chartName
}

func (as *AddonsSuite) doPrometheusUpdate(addonName string, values map[string]interface{}) {
	path := fmt.Sprintf("/var/lib/mke/manifests/helm/addon_crd_manifest_%s.yaml", addonName)
	valuesBytes, err := yaml.Marshal(values)
	as.Require().NoError(err)
	tw := util.TemplateWriter{
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
			Version:   "11.16.8",
			TargetNS:  "default",
		},
	}
	buf := bytes.NewBuffer([]byte{})
	as.Require().NoError(tw.WriteToBuffer(buf))

	as.putFile(path, buf.String())
}

func (as *AddonsSuite) getKubeConfig(node string) *restclient.Config {
	machine, err := as.MachineForName(node)
	as.Require().NoError(err)
	ssh, err := as.SSH(node)
	as.Require().NoError(err)
	kubeConf, err := ssh.ExecWithOutput("cat /var/lib/mke/pki/admin.conf")
	as.Require().NoError(err)
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	as.Require().NoError(err)
	hostPort, err := machine.HostPort(6443)
	as.Require().NoError(err)
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg
}

func TestAddonsSuite(t *testing.T) {

	s := AddonsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}

	suite.Run(t, &s)

}

func (as *AddonsSuite) prepareConfigWithAddons(addonName string) {
	as.putFile("/tmp/mke.yaml", fmt.Sprintf(mkeConfigWithAddon, addonName))
}

func (as *AddonsSuite) putFile(path string, content string) {
	controllerNode := fmt.Sprintf("controller%d", 0)
	ssh, err := as.SSH(controllerNode)
	as.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", content, path))

	as.Require().NoError(err)
}

const mkeConfigWithAddon = `
helm:
  repositories:
  - name: stable
    url: https://charts.helm.sh/stable
  - name: prometheus-community
    url: https://prometheus-community.github.io/helm-charts
  addons:
  - name: %s
    chartname: prometheus-community/prometheus
    version: "11.16.8"
    values: ""
    namespace: default
`

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
