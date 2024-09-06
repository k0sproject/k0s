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
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/inttest/common"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/constant"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/suite"
)

type AddonsSuite struct {
	common.BootlooseSuite
}

func (as *AddonsSuite) TestHelmBasedAddons() {
	ctx := as.Context()
	crlog.SetLogger(testr.New(as.T()))

	addonName := "test-addon"
	ociAddonName := "oci-addon"
	fileAddonName := "tgz-addon"
	as.PutFile(as.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithAddon, addonName))
	as.pullHelmChart(as.ControllerNode(0))
	as.Require().NoError(as.InitController(0, "--config=/tmp/k0s.yaml", "--enable-dynamic-config"))
	as.NoError(as.RunWorkers())
	kc, err := as.KubeClient(as.ControllerNode(0))
	as.Require().NoError(err)
	err = as.WaitForNodeReady(as.WorkerNode(0), kc)
	as.NoError(err)
	as.waitForTestRelease(addonName, "0.4.0", "default", 1)
	as.waitForTestRelease(ociAddonName, "0.6.0", "default", 1)
	as.waitForTestRelease(fileAddonName, "0.6.0", "kube-system", 1)

	as.AssertSomeKubeSystemPods(kc)

	as.Run("Rename chart in Helm extension", func() { as.renameChart(ctx) })

	values := map[string]interface{}{
		"replicaCount": 2,
		"image": map[string]interface{}{
			"pullPolicy": "Always",
		},
	}
	as.doTestAddonUpdate(addonName, values)
	chart := as.waitForTestRelease(addonName, "0.6.0", "default", 2)
	as.Require().NoError(as.checkCustomValues(chart.Status.ReleaseName))
	as.deleteRelease(chart)
	as.deleteUninstalledChart(ctx)
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

func (as *AddonsSuite) renameChart(ctx context.Context) {
	restConfig, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	k0sClients, err := k0sclientset.NewForConfig(restConfig)
	as.Require().NoError(err)

	configs := k0sClients.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)
	cfg, err := configs.Get(ctx, constant.ClusterConfigObjectName, metav1.GetOptions{})
	as.Require().NoError(err)

	i := slices.IndexFunc(cfg.Spec.Extensions.Helm.Charts, func(c k0sv1beta1.Chart) bool {
		return c.Name == "tgz-addon"
	})
	as.Require().GreaterOrEqual(i, 0, "Didn't find tgz-addon in %v", cfg.Spec.Extensions.Helm.Charts)
	cfg.Spec.Extensions.Helm.Charts[i].Name = "tgz-renamed-addon"

	cfg, err = configs.Update(ctx, cfg, metav1.UpdateOptions{FieldManager: as.T().Name()})
	as.Require().NoError(err)
	if data, err := yaml.Marshal(cfg); as.NoError(err) {
		as.T().Logf("%s", data)
	}

	as.Require().NoError(wait.PollUntilContextCancel(ctx, 350*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		charts, err := k0sClients.HelmV1beta1().Charts(constant.ClusterConfigNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}

		hasChart := func(name string) bool {
			return slices.IndexFunc(charts.Items, func(c helmv1beta1.Chart) bool {
				return c.Name == name
			}) >= 0
		}

		return !hasChart("k0s-addon-chart-tgz-addon") && hasChart("k0s-addon-chart-tgz-renamed-addon"), nil
	}), "While waiting for Chart resource to be swapped")

	as.waitForTestRelease("tgz-renamed-addon", "0.6.0", "kube-system", 1)
}

func (as *AddonsSuite) deleteRelease(chart *helmv1beta1.Chart) {
	ctx := as.Context()
	as.T().Logf("Deleting chart %s/%s", chart.Namespace, chart.Name)
	ssh, err := as.SSH(ctx, as.ControllerNode(0))
	as.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(ctx, "rm /var/lib/k0s/manifests/helm/0_helm_extension_test-addon.yaml")
	as.Require().NoError(err)
	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)
	k8sclient, err := k8s.NewForConfig(cfg)
	as.Require().NoError(err)
	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(pollCtx context.Context) (done bool, err error) {
		as.T().Logf("Expecting have no secrets left for release %s/%s", chart.Status.Namespace, chart.Status.ReleaseName)
		items, err := k8sclient.CoreV1().Secrets(chart.Status.Namespace).List(pollCtx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("name=%s", chart.Status.ReleaseName),
		})
		if err != nil {
			if ctxErr := context.Cause(ctx); ctxErr != nil {
				return false, errors.Join(err, ctxErr)
			}
			as.T().Log("Error while listing secrets:", err)
			return false, nil
		}
		if len(items.Items) > 0 {
			return false, nil
		}
		as.T().Log("Release uninstalled successfully")
		return true, nil
	}))

	chartClient, err := client.New(cfg, client.Options{Scheme: k0sscheme.Scheme})
	as.Require().NoError(err)

	as.T().Logf("Expecting chart %s/%s to be deleted", chart.Namespace, chart.Name)
	var lastResourceVersion string
	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		var found helmv1beta1.Chart
		err := chartClient.Get(ctx, client.ObjectKey{Namespace: chart.Namespace, Name: chart.Name}, &found)
		switch {
		case err == nil:
			if lastResourceVersion == "" || lastResourceVersion != found.ResourceVersion {
				as.T().Log("Chart not yet deleted")
				lastResourceVersion = found.ResourceVersion
			}
			return false, nil

		case apierrors.IsNotFound(err):
			as.T().Log("Chart has been deleted")
			return true, nil

		default:
			as.T().Log("Error while getting chart: ", err)
			return false, nil
		}
	}))
}

func (as *AddonsSuite) deleteUninstalledChart(ctx context.Context) {
	spec := helmv1beta1.ChartSpec{
		ChartName:   "whatever",
		ReleaseName: "nonexistent",
		Namespace:   "default",
		Version:     "1",
	}
	status := helmv1beta1.ChartStatus{
		ReleaseName: spec.ReleaseName,
		Namespace:   spec.Namespace,
		Version:     spec.Version,
		AppVersion:  "1",
		Revision:    1,
		ValuesHash:  spec.HashValues(),
	}
	chart := &helmv1beta1.Chart{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "bogus",
			Namespace:  "kube-system",
			Finalizers: []string{"helm.k0sproject.io/uninstall-helm-release"},
		},
	}

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)

	crClient, err := client.New(cfg, client.Options{Scheme: k0sscheme.Scheme})
	as.Require().NoError(err)

	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		if _, err := controllerutil.CreateOrUpdate(ctx, crClient, chart, func() error {
			chart.Spec = spec
			return nil
		}); err != nil {
			as.T().Log("Failed to create bogus chart resource: ", err)
			return false, nil
		}
		chart.Status = status
		if err := crClient.Status().Update(ctx, chart); err != nil {
			as.T().Log("Failed to update bogus chart resource's status: ", err)
			return false, nil
		}

		as.T().Log("Created bogus chart")
		return true, nil
	}))

	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := crClient.Delete(ctx, chart); err != nil {
			as.T().Log("Failed to delete bogus chart resource: ", err)
			return false, nil
		}

		as.T().Log("Deleted bogus chart")
		return true, nil
	}))

	as.T().Logf("Expecting bogus chart %s/%s to be deleted", chart.Namespace, chart.Name)
	var lastResourceVersion string
	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		var found helmv1beta1.Chart
		err := crClient.Get(ctx, client.ObjectKey{Namespace: chart.Namespace, Name: chart.Name}, &found)
		switch {
		case err == nil:
			if lastResourceVersion != found.ResourceVersion {
				as.T().Log("Bogus chart not yet deleted")
				lastResourceVersion = found.ResourceVersion
			}
			return false, nil

		case apierrors.IsNotFound(err):
			as.T().Log("Bogus chart has been deleted")
			return true, nil

		default:
			as.T().Log("Error while getting bogus chart: ", err)
			return false, nil
		}
	}))
}

func (as *AddonsSuite) waitForTestRelease(addonName, appVersion string, namespace string, rev int64) *helmv1beta1.Chart {
	ctx := as.Context()
	as.T().Logf("waiting to see %s release ready in kube API, generation %d", addonName, rev)

	cfg, err := as.GetKubeConfig(as.ControllerNode(0))
	as.Require().NoError(err)

	chartClient, err := client.New(cfg, client.Options{Scheme: k0sscheme.Scheme})
	as.Require().NoError(err)
	var chart helmv1beta1.Chart
	var lastResourceVersion string
	as.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(pollCtx context.Context) (done bool, err error) {
		err = chartClient.Get(pollCtx, client.ObjectKey{
			Namespace: "kube-system",
			Name:      fmt.Sprintf("k0s-addon-chart-%s", addonName),
		}, &chart)
		if err != nil {
			if ctxErr := context.Cause(ctx); ctxErr != nil {
				return false, errors.Join(err, ctxErr)
			}
			as.T().Log("Error while querying for chart:", err)
			return false, nil
		}
		if lastResourceVersion != "" && lastResourceVersion == chart.ResourceVersion {
			return false, nil // That version has already been inspected.
		}

		var errs []string
		if chart.Status.ReleaseName == "" {
			errs = append(errs, "no release name")
		}
		if chart.Generation != rev {
			errs = append(errs, fmt.Sprintf("expected generation to be %d, but was %d", rev, chart.Generation))
		}
		if chart.Status.Revision != rev {
			errs = append(errs, fmt.Sprintf("expected revision to be %d, but was %d", rev, chart.Status.Revision))
		}
		if chart.Status.Error != "" {
			errs = append(errs, fmt.Sprintf("expected error to be empty, but was %q", chart.Status.Error))
		}

		lastResourceVersion = chart.ResourceVersion
		if len(errs) > 0 {
			as.T().Logf("Test addon release doesn't meet criteria yet (version %q): %s", lastResourceVersion, strings.Join(errs, "; "))
			return false, nil
		}

		as.Require().Equal(namespace, chart.Status.Namespace)
		as.Require().Equal(appVersion, chart.Status.AppVersion)
		as.Require().Equal(namespace, chart.Status.Namespace)
		as.Require().NotEmpty(chart.Status.ReleaseName)
		as.Require().Equal(rev, chart.Status.Revision)
		as.T().Log("Found test addon release:", chart.Name)
		as.Require().Equal(rev, chart.Generation)
		return true, nil
	}))
	return &chart
}

func (as *AddonsSuite) checkCustomValues(releaseName string) error {
	ctx := as.Context()
	as.T().Logf("waiting to see release to have values set from CRD yaml")
	kc, err := as.KubeClient(as.ControllerNode(0))
	if err != nil {
		return err
	}
	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(pollCtx context.Context) (done bool, err error) {
		serverDeployment := fmt.Sprintf("%s-echo-server", releaseName)
		d, err := kc.AppsV1().Deployments("default").Get(pollCtx, serverDeployment, metav1.GetOptions{})
		if err != nil {
			if ctxErr := context.Cause(ctx); ctxErr != nil {
				return false, errors.Join(err, ctxErr)
			}
			as.T().Log("Error while getting Deployment:", err)
			return false, nil
		}
		as.Require().Equal(int32(2), *d.Spec.Replicas, "Must have replicas value set from passed values")
		as.Require().Equal("Always", string(d.Spec.Template.Spec.Containers[0].ImagePullPolicy))
		return true, nil
	})
}

func (as *AddonsSuite) doTestAddonUpdate(addonName string, values map[string]interface{}) {
	path := fmt.Sprintf("/var/lib/k0s/manifests/helm/0_helm_extension_%s.yaml", addonName)
	valuesBytes, err := yaml.Marshal(values)
	as.Require().NoError(err)
	tw := templatewriter.TemplateWriter{
		Name:     "testChartUpdate",
		Template: chartCrdTemplate,
		Data: struct {
			Name         string
			ChartName    string
			Values       string
			Version      string
			TargetNS     string
			ForceUpgrade *bool
		}{
			Name:         "test-addon",
			ChartName:    "ealenn/echo-server",
			Values:       string(valuesBytes),
			Version:      "0.5.0",
			TargetNS:     "default",
			ForceUpgrade: ptr.To(false),
		},
	}
	buf := bytes.NewBuffer([]byte{})
	as.Require().NoError(tw.WriteToBuffer(buf))

	as.PutFile(as.ControllerNode(0), path, buf.String())
}

func TestAddonsSuite(t *testing.T) {

	s := AddonsSuite{
		common.BootlooseSuite{
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
            forceUpgrade: false
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
{{- if ne .ForceUpgrade nil }}
  forceUpgrade: {{ .ForceUpgrade }}
{{- end }}
`
