// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/inttest/common"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	v1 "k8s.io/api/core/v1"
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

const (
	registryCACertContainerPath = "/tmp/registry-ca.crt"
	echoServerTgzPath           = "./testdata/chart/echo-server-0.5.0.tgz"
)

type AddonsSuite struct {
	common.BootlooseSuite

	registryCABytes []byte
}

func initCA(t *testing.T) (cert *x509.Certificate, key crypto.Signer) {
	certData, _, keyData, err := initca.New(&csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         "Test Registry CA",
	})
	require.NoError(t, err)

	cert, err = helpers.ParseCertificatePEM(certData)
	require.NoError(t, err)

	key, err = helpers.ParsePrivateKeyPEM(keyData)
	require.NoError(t, err)

	return
}

func issueServerCertsWithSelfSignedCA(t *testing.T, certsDir string) []byte {
	caCert, caKey := initCA(t)

	s, err := local.NewSigner(caKey, caCert, signer.DefaultSigAlgo(caKey), &config.Signing{
		Default: &config.SigningProfile{
			Usage: []string{
				"digital signature",
				"key encipherment",
				"server auth",
			},
			Expiry:       helpers.OneDay,
			ExpiryString: helpers.OneDay.String(),
		},
	})
	require.NoError(t, err)

	serverCertCSR, serverKey, err := csr.ParseRequest(&csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         "Test Registry",
		Hosts:      []string{"host.docker.internal"},
	})
	require.NoError(t, err)

	serverCert, err := s.Sign(signer.SignRequest{Request: string(serverCertCSR)})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path.Join(certsDir, "tls.crt"), serverCert, 0644))
	require.NoError(t, os.WriteFile(path.Join(certsDir, "tls.key"), serverKey, 0600))

	return serverCert
}

// pushChartToLocalRegistry pushes a pre-downloaded echo-server chart to the local registry
func (as *AddonsSuite) pushChartToLocalRegistry() {
	helmEnv := cli.New()

	cfg := &action.Configuration{}
	err := cfg.Init(helmEnv.RESTClientGetter(), "", "memory", as.T().Logf)
	as.Require().NoError(err)

	pushAction := action.NewPushWithOpts(
		action.WithPushConfig(cfg),
		action.WithPushOptWriter(os.Stdout),
		action.WithInsecureSkipTLSVerify(true),
	)
	pushAction.Settings = helmEnv

	_, err = pushAction.Run(echoServerTgzPath, fmt.Sprintf("oci://localhost:%d/charts", as.GetRegistryHostPort()))
	as.Require().NoError(err)
}

// uploadRegistryCAToControllers uploads the CA certificate of the local registry to all controller nodes
func (as *AddonsSuite) uploadRegistryCAToControllers() {
	for i := range as.ControllerCount {
		as.PutFile(as.ControllerNode(i), registryCACertContainerPath, string(as.registryCABytes))
	}
}

func (as *AddonsSuite) SetupTest() {
	as.pushChartToLocalRegistry()
	as.uploadRegistryCAToControllers()
}

func (as *AddonsSuite) TestHelmBasedAddons() {
	ctx := as.Context()
	crlog.SetLogger(testr.New(as.T()))

	addonName := "test-addon"
	ociAddonName := "oci-addon"
	fileAddonName := "tgz-addon"
	selfSignedOCIAddonName := "self-signed-oci-addon"

	p := k0sConfigParams{
		BasicAddonName:      addonName,
		LocalRegistryCAPath: registryCACertContainerPath,
		LocalRegistryHost:   "host.docker.internal",
		LocalRegistryPort:   as.GetRegistryHostPort(),
	}

	buf := new(bytes.Buffer)
	as.Require().NoError(k0sConfigWithAddonTemplate.Execute(buf, p))
	as.PutFile(as.ControllerNode(0), "/tmp/k0s.yaml", buf.String())
	as.pullHelmChart(as.ControllerNode(0))
	as.Require().NoError(as.InitController(0, "--config=/tmp/k0s.yaml", "--enable-dynamic-config"))
	as.NoError(as.RunWorkers())
	kc, err := as.KubeClient(as.ControllerNode(0))
	as.Require().NoError(err)
	err = as.WaitForNodeReady(as.WorkerNode(0), kc)
	as.NoError(err)
	as.waitForTestRelease(addonName, "0.4.0", metav1.NamespaceDefault, 1)
	as.waitForTestRelease(ociAddonName, "0.6.0", metav1.NamespaceDefault, 1)
	as.waitForTestRelease(fileAddonName, "0.6.0", metav1.NamespaceSystem, 1)
	as.waitForTestRelease(selfSignedOCIAddonName, "0.6.0", metav1.NamespaceDefault, 1)

	// TODO Check that the authenticated chart is in pending state before adding the secret

	// Add secret for authed-echo-server chart
	as.T().Log("Adding secret for authorized repo and waiting for it's chart to re-reconcile")
	_, err = kc.CoreV1().Secrets(metav1.NamespaceDefault).Create(ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic-auth-secret",
		},
		Type: v1.SecretTypeBasicAuth,
		StringData: map[string]string{
			v1.BasicAuthUsernameKey: "foo",
			v1.BasicAuthPasswordKey: "bar",
		},
	}, metav1.CreateOptions{})
	as.NoError(err)
	as.waitForTestRelease("authed-echo-server", "0.4.0", metav1.NamespaceDefault, 1)

	as.AssertSomeKubeSystemPods(kc)

	as.Run("Rename chart in Helm extension", func() { as.renameChart(ctx) })

	values := map[string]any{
		"replicaCount": 2,
		"image": map[string]any{
			"pullPolicy": "Always",
		},
	}
	as.doTestAddonUpdate(addonName, values)
	chart := as.waitForTestRelease(addonName, "0.6.0", metav1.NamespaceDefault, 2)
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
	as.Require().GreaterOrEqualf(i, 0, "Didn't find tgz-addon in %v", cfg.Spec.Extensions.Helm.Charts)
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

	as.waitForTestRelease("tgz-renamed-addon", "0.6.0", metav1.NamespaceSystem, 1)
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
			LabelSelector: "name=" + chart.Status.ReleaseName,
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
		Namespace:   metav1.NamespaceDefault,
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
			Namespace:  metav1.NamespaceSystem,
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
			Namespace: metav1.NamespaceSystem,
			Name:      "k0s-addon-chart-" + addonName,
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
		serverDeployment := releaseName + "-echo-server"
		d, err := kc.AppsV1().Deployments(metav1.NamespaceDefault).Get(pollCtx, serverDeployment, metav1.GetOptions{})
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

func (as *AddonsSuite) doTestAddonUpdate(addonName string, values map[string]any) {
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
			TargetNS:     metav1.NamespaceDefault,
			ForceUpgrade: ptr.To(false),
		},
	}
	buf := bytes.NewBuffer([]byte{})
	as.Require().NoError(tw.WriteToBuffer(buf))

	as.PutFile(as.ControllerNode(0), path, buf.String())
}

func TestAddonsSuite(t *testing.T) {
	registryTLSDir := path.Join(t.TempDir(), "registry-tls")
	require.NoError(t, os.MkdirAll(registryTLSDir, 0755))

	s := AddonsSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			WithRegistry:    true,
			RegistryTLSPath: registryTLSDir,
		},
		registryCABytes: issueServerCertsWithSelfSignedCA(t, registryTLSDir),
	}

	suite.Run(t, &s)
}

type k0sConfigParams struct {
	BasicAddonName string

	LocalRegistryCAPath string
	LocalRegistryHost   string
	LocalRegistryPort   int
}

const k0sConfigWithAddonRawTemplate = `
spec:
    extensions:
        helm:
          repositories:
          - name: ealenn
            url: https://ealenn.github.io/charts
          - name: auth-test
            url: https://ealenn.github.io/charts
            credentialsFrom:
              secretRef:
                name: basic-auth-secret
                namespace: default
          - name: oci
            url: oci://ghcr.io/makhov/k0s-charts
          - name: self-signed-oci
            url: oci://{{ .LocalRegistryHost }}:{{ .LocalRegistryPort }}
            caFile: {{ .LocalRegistryCAPath }}
          charts:
          - name: {{ .BasicAddonName }}
            chartname: ealenn/echo-server
            version: "0.3.1"
            values: ""
            namespace: default
          - name: oci-addon
            chartname: oci://ghcr.io/makhov/k0s-charts/echo-server
            version: "0.5.0"
            values: ""
            namespace: default
          - name: self-signed-oci-addon
            chartname: oci://{{ .LocalRegistryHost }}:{{ .LocalRegistryPort }}/charts/echo-server
            version: "0.5.0"
            values: ""
            namespace: default
          - name: tgz-addon
            chartname: /tmp/chart.tgz
            version: "0.0.1"
            values: ""
            namespace: kube-system
            forceUpgrade: false
          - name: authed-echo-server
            chartname: auth-test/echo-server
            version: "0.3.1"
            values: ""
            namespace: default
`

var k0sConfigWithAddonTemplate = template.Must(template.New("k0sConfigWithAddon").Parse(k0sConfigWithAddonRawTemplate))

// TODO: this actually duplicates logic from the controller code
// better to somehow handle it by programmatic api
const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: ` + metav1.NamespaceSystem + `
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
