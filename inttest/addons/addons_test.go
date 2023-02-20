/*
Copyright 2020 k0s authors

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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/helm.k0sproject.io/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type AddonsSuite struct {
	common.FootlooseSuite
}

func (as *AddonsSuite) TestHelmBasedAddons() {
	addonName := "test-addon"
	ociAddonName := "oci-addon"
	fileAddonName := "tgz-addon"
	as.PutFile(as.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithAddon, addonName))
	as.pullHelmChart(as.ControllerNode(0))
	as.Require().NoError(as.InitController(0, "--config=/tmp/k0s.yaml"))
	as.NoError(as.RunWorkers())
	kc, err := as.KubeClient(as.ControllerNode(0))
	as.Require().NoError(err)
	err = as.WaitForNodeReady(as.WorkerNode(0), kc)
	as.NoError(err)
	as.waitForTestRelease(addonName, "0.4.0", "default", 1)
	as.waitForTestRelease(ociAddonName, "0.6.0", "default", 1)
	as.waitForTestRelease(fileAddonName, "0.6.0", "kube-system", 1)

	as.AssertSomeKubeSystemPods(kc)

	values := map[string]interface{}{
		"replicaCount": 2,
		"image": map[string]interface{}{
			"pullPolicy": "Always",
		},
	}
	as.doTestAddonUpdate(addonName, values)
	chart := as.waitForTestRelease(addonName, "0.4.0", "default", 2)
	as.Require().NoError(as.checkCustomValues(chart.Status.ReleaseName))
	as.doPrometheusDelete(chart)
}

func (as *AddonsSuite) pullHelmChart(node string) {
	ctx := as.Context()
	ssh, err := as.SSH(ctx, node)
	as.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(ctx, "helm repo add ealenn https://ealenn.github.io/charts")
	as.Require().NoError(err)
	_, err = ssh.ExecWithOutput(ctx, "helm pull --destination /tmp ealenn/echo-server")
	as.Require().NoError(err)
	_, err = ssh.ExecWithOutput(ctx, "mv /tmp/echo-server* /tmp/chart.tgz")
	as.Require().NoError(err)
}

func (as *AddonsSuite) doPrometheusDelete(chart *v1beta1.Chart) {
	as.T().Logf("Deleting chart %s/%s", chart.Namespace, chart.Name)
	ssh, err := as.SSH(as.Context(), as.ControllerNode(0))
	as.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(as.Context(), "rm /var/lib/k0s/manifests/helm/addon_crd_manifest_test-addon.yaml")
	as.Require().NoError(err)

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	k8sclient, err := k8s.NewForConfig(cfg)
	as.Require().NoError(err)
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		as.T().Logf("Expecting have no secrets left for release %s/%s", chart.Namespace, chart.Name)
		items, err := k8sclient.CoreV1().Secrets("default").List(as.Context(), v1.ListOptions{})
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

func (as *AddonsSuite) waitForTestRelease(addonName, appVersion string, namespace string, rev int64) *v1beta1.Chart {
	as.T().Logf("waiting to see test-addon release ready in kube API, generation %d", rev)

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	err = v1beta1.AddToScheme(scheme.Scheme)
	as.Require().NoError(err)
	chartClient, err := client.New(cfg, client.Options{
		Scheme: scheme.Scheme,
	})
	as.Require().NoError(err)
	var chart v1beta1.Chart
	as.Require().NoError(wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		err = chartClient.Get(as.Context(), client.ObjectKey{
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

		as.Require().Equal(namespace, chart.Status.Namespace)
		as.Require().Equal(appVersion, chart.Status.AppVersion)
		as.Require().Equal(namespace, chart.Status.Namespace)
		as.Require().NotEmpty(chart.Status.ReleaseName)
		as.Require().Empty(chart.Status.Error)
		as.Require().Equal(rev, chart.Status.Revision)
		as.T().Logf("found test addon release: %s\n", chart.Name)
		as.Require().Equal(rev, chart.Generation)
		return true, nil
	}))
	return &chart
}

func (as *AddonsSuite) checkCustomValues(releaseName string) error {
	as.T().Logf("waiting to see release to have values set from CRD yaml")
	kc, err := as.KubeClient(as.ControllerNode(0))
	if err != nil {
		return err
	}
	return wait.PollImmediate(time.Second, 2*time.Minute, func() (done bool, err error) {
		serverDeployment := fmt.Sprintf("%s-echo-server", releaseName)
		d, err := kc.AppsV1().Deployments("default").Get(as.Context(), serverDeployment, v1.GetOptions{})
		if err != nil {
			return false, nil
		}
		as.Require().Equal(int32(2), *d.Spec.Replicas, "Must have replicas value set from passed values")
		as.Require().Equal("Always", string(d.Spec.Template.Spec.Containers[0].ImagePullPolicy))
		return true, nil
	})
}

func (as *AddonsSuite) doTestAddonUpdate(addonName string, values map[string]interface{}) {
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
			ChartName: "ealenn/echo-server",
			Values:    string(valuesBytes),
			Version:   "0.4.0",
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
			WorkerCount:     1,
		},
	}

	suite.Run(t, &s)

}

const k0sConfigWithAddon = `
spec:
    extensions:
        helm:
          repositories:
          - name: ealenn
            url: https://ealenn.github.io/charts
          charts:
          - name: %s
            chartname: ealenn/echo-server
            version: "0.3.1"
            values: ""
            namespace: default
          - name: oci-addon
            chartname: oci://ghcr.io/makhov/k0s-charts/echo-server
            version: "0.5.0"
            values: ""
            namespace: default
          - name: tgz-addon
            chartname: /tmp/chart.tgz
            version: "0.0.1"
            values: ""
            namespace: kube-system
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
