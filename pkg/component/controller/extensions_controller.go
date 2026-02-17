// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/helm"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	apiretry "k8s.io/client-go/util/retry"

	"github.com/avast/retry-go"
	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const HelmExtensionStackName = "helm"

// repositoryCache provides thread-safe access to Helm repository configurations
type repositoryCache struct {
	cache map[string]k0sv1beta1.Repository
	mutex sync.RWMutex
}

// get retrieves a repository configuration by key (repo name for traditional, hostname for OCI)
// Returns nil if not found.
func (rc *repositoryCache) get(key string) *k0sv1beta1.Repository {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	repo, ok := rc.cache[key]
	if !ok {
		return nil
	}
	return &repo
}

// update replaces the entire cache contents with the provided repositories
func (rc *repositoryCache) update(repositories []k0sv1beta1.Repository) {
	// Build a new cache and swap it out under lock
	newCache := make(map[string]k0sv1beta1.Repository)

	// Populate with new repositories
	// OCI repos are keyed by hostname, traditional repos by name
	var repoName string
	for _, repo := range repositories {
		if registry.IsOCI(repo.URL) {
			// Extract hostname from OCI URL and use as key
			url, err := url.Parse(repo.URL)
			// This shouldn't really happen at this point but just in case let's log it
			if err != nil {
				logrus.WithError(err).Warnf("Invalid repository URL %q, skipping", repo.URL)
				continue
			}
			repoName = url.Host
		} else {
			// Traditional repo: use name as key
			repoName = repo.Name
		}
		newCache[repoName] = repo
	}

	rc.mutex.Lock()
	rc.cache = newCache
	defer rc.mutex.Unlock()

}

// Helm watch for Chart crd
type ExtensionsController struct {
	L               *logrus.Entry
	kubeConfig      string
	leaderElector   leaderelector.Interface
	manifestsDir    string
	stop            context.CancelFunc
	repositoryCache *repositoryCache
}

var _ manager.Component = (*ExtensionsController)(nil)
var _ manager.Reconciler = (*ExtensionsController)(nil)

// NewExtensionsController builds new HelmAddons
func NewExtensionsController(k0sVars *config.CfgVars, kubeClientFactory kubeutil.ClientFactoryInterface, leaderElector leaderelector.Interface) *ExtensionsController {
	return &ExtensionsController{
		L:               logrus.WithFields(logrus.Fields{"component": "extensions_controller"}),
		kubeConfig:      k0sVars.AdminKubeConfigPath,
		leaderElector:   leaderElector,
		manifestsDir:    filepath.Join(k0sVars.ManifestsDir, "helm"),
		repositoryCache: &repositoryCache{},
	}
}

const (
	namespaceToWatch = metav1.NamespaceSystem
)

// Run runs the extensions controller
func (ec *ExtensionsController) Reconcile(ctx context.Context, clusterConfig *k0sv1beta1.ClusterConfig) error {
	ec.L.Info("Extensions reconciliation started")
	defer ec.L.Info("Extensions reconciliation finished")
	return ec.reconcileHelmExtensions(clusterConfig.Spec.Extensions.Helm)
}

// reconcileHelmExtensions creates instance of Chart CR for each chart of the config file
// it also reconciles repositories settings
// the actual helm install/update/delete management is done by ChartReconciler structure
func (ec *ExtensionsController) reconcileHelmExtensions(helmSpec *k0sv1beta1.HelmExtensions) error {
	if helmSpec == nil {
		return nil
	}

	// Update repository cache
	ec.repositoryCache.update(helmSpec.Repositories)

	var errs []error
	var fileNamesToKeep []string
	for _, chart := range helmSpec.Charts {
		fileName := chartManifestFileName(&chart)
		fileNamesToKeep = append(fileNamesToKeep, fileName)

		path, err := ec.writeChartManifestFile(chart, fileName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't write file for Helm chart manifest %q: %w", chart.ChartName, err))
			continue
		}

		ec.L.Infof("Wrote Helm chart manifest file %q", path)
	}

	if err := filepath.WalkDir(ec.manifestsDir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case !entry.Type().IsRegular():
			ec.L.Debugf("Keeping %v as it is not a regular file", entry)
		case slices.Contains(fileNamesToKeep, entry.Name()):
			ec.L.Debugf("Keeping %v as it belongs to a known Helm extension", entry)
		case !isChartManifestFileName(entry.Name()):
			ec.L.Debugf("Keeping %v as it is not a Helm chart manifest file", entry)
		default:
			if err := os.Remove(path); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					errs = append(errs, fmt.Errorf("failed to remove Helm chart manifest file, the Chart resource will remain in the cluster: %w", err))
				}
			} else {
				ec.L.Infof("Removed Helm chart manifest file %q", path)
			}
		}

		return nil
	}); err != nil {
		errs = append(errs, fmt.Errorf("failed to walk Helm chart manifest directory: %w", err))
	}

	return errors.Join(errs...)
}

func (ec *ExtensionsController) writeChartManifestFile(chart k0sv1beta1.Chart, fileName string) (string, error) {
	// Find matching repository for this chart
	var repository *k0sv1beta1.Repository
	if repoID := extractRepositoryIdentifier(chart.ChartName); repoID != "" {
		repository = ec.repositoryCache.get(repoID)
	}

	tw := templatewriter.TemplateWriter{
		Path:     filepath.Join(ec.manifestsDir, fileName),
		Name:     "addon_crd_manifest",
		Template: chartCrdTemplate,
		Data: struct {
			k0sv1beta1.Chart
			Finalizer  string
			Repository *k0sv1beta1.Repository
		}{
			Chart:      chart,
			Finalizer:  finalizerName,
			Repository: repository,
		},
	}
	if err := tw.Write(); err != nil {
		return "", err
	}
	return tw.Path, nil
}

// Determines the file name to use when storing a chart as a manifest on disk.
func chartManifestFileName(c *k0sv1beta1.Chart) string {
	return fmt.Sprintf("%d_helm_extension_%s.yaml", c.Order, c.Name)
}

// Determines if the given file name is in the format for chart manifest file names.
func isChartManifestFileName(fileName string) bool {
	return regexp.MustCompile(`^-?[0-9]+_helm_extension_.+\.yaml$`).MatchString(fileName)
}

type ChartReconciler struct {
	client.Client
	kubeConfig      string
	repositoryCache *repositoryCache
	leaderElector   leaderelector.Interface
	L               *logrus.Entry
}

func (cr *ChartReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if !cr.leaderElector.IsLeader() {
		return reconcile.Result{}, nil
	}

	var chartInstance helmv1beta1.Chart

	if err := cr.Get(ctx, req.NamespacedName, &chartInstance); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !chartInstance.DeletionTimestamp.IsZero() {
		cr.L.Debugf("Uninstall reconciliation request: %s", req)
		// uninstall chart
		if err := cr.uninstall(ctx, chartInstance); err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				return reconcile.Result{}, fmt.Errorf("can't uninstall chart: %w", err)
			}

			cr.L.Debugf("No Helm release found for chart %s, assuming it has already been uninstalled", req)
		}

		if err := removeFinalizer(ctx, cr.Client, &chartInstance); err != nil {
			return reconcile.Result{}, fmt.Errorf("while trying to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}
	cr.L.Debugf("Install or update reconciliation request: %s", req)
	if err := cr.updateOrInstallChart(ctx, chartInstance); err != nil {
		return reconcile.Result{}, fmt.Errorf("can't update or install chart: %w", err)
	}

	cr.L.Debugf("Installed or updated reconciliation request: %s", req)
	return reconcile.Result{}, nil
}

func (cr *ChartReconciler) uninstall(ctx context.Context, chart helmv1beta1.Chart) error {
	// Create ephemeral Helm commands without repository (uninstall doesn't need it)
	helmCmd, cleanup, err := helm.NewCommands(cr.kubeConfig, nil)
	if err != nil {
		return fmt.Errorf("can't create Helm commands: %w", err)
	}
	defer cleanup()

	if err := helmCmd.UninstallRelease(ctx, chart.Status.ReleaseName, chart.Status.Namespace); err != nil {
		return fmt.Errorf("can't uninstall release `%s/%s`: %w", chart.Status.Namespace, chart.Status.ReleaseName, err)
	}
	return nil
}

func removeFinalizer(ctx context.Context, c client.Client, chart *helmv1beta1.Chart) error {
	idx := slices.Index(chart.Finalizers, finalizerName)
	if idx < 0 {
		return nil
	}

	path := fmt.Sprintf("/metadata/finalizers/%d", idx)
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value,omitempty"`
	}{
		{"test", path, finalizerName},
		{"remove", path, ""},
	})
	if err != nil {
		return err
	}

	return c.Patch(ctx, chart, client.RawPatch(types.JSONPatchType, patch))
}

const defaultTimeout = 10 * time.Minute

// loadAndMergeRepositoryConfig loads repository configuration from a secret and merges it
// with inline configuration. Secret values take precedence over inline values.
//
// Supported secret keys:
//   - url: Repository URL
//   - username, password: Basic auth credentials
//   - ca.crt: CA certificate for HTTPS verification
//   - tls.crt, tls.key: Client certificate/key for mTLS
//   - insecure: "true" to skip TLS verification
//
// Returns the merged repository configuration with in-memory certificate data.
func (cr *ChartReconciler) loadAndMergeRepositoryConfig(ctx context.Context, chart helmv1beta1.Chart) (*helm.Repository, error) {
	repoSpec := chart.Spec.Repository
	if repoSpec == nil {
		return nil, errors.New("chart has no repository configuration")
	}

	// If no secret reference, just convert inline config
	if repoSpec.ConfigFrom == nil || repoSpec.ConfigFrom.SecretRef == nil {
		repo := repoSpec.ToHelm(extractRepositoryIdentifier(chart.Spec.ChartName))
		return &repo, nil
	}

	secretRef := repoSpec.ConfigFrom.SecretRef
	secretNamespace := secretRef.Namespace
	if secretNamespace == "" {
		secretNamespace = chart.Namespace
	}

	// Fetch the secret
	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      secretRef.Name,
		Namespace: secretNamespace,
	}
	if err := cr.Get(ctx, secretKey, &secret); err != nil {
		return nil, fmt.Errorf("failed to get repository config secret %s: %w", secretKey, err)
	}

	// Start with inline configuration
	repo := repoSpec.ToHelm(extractRepositoryIdentifier(chart.Spec.ChartName))

	// Override with values from secret (secret takes precedence)
	if val, ok := secret.Data["url"]; ok && len(val) > 0 {
		repo.URL = string(val)
	}
	if val, ok := secret.Data["username"]; ok && len(val) > 0 {
		repo.Username = string(val)
	}
	if val, ok := secret.Data["password"]; ok && len(val) > 0 {
		repo.Password = string(val)
	}

	// Handle certificate data - use in-memory data (no temp files needed here!)
	// Use standard Kubernetes TLS secret keys (ca.crt, tls.crt, tls.key)
	if caData, ok := secret.Data["ca.crt"]; ok && len(caData) > 0 {
		repo.CAData = caData
		repo.CAFile = "" // Clear file path if data is provided
	}

	if certData, ok := secret.Data["tls.crt"]; ok && len(certData) > 0 {
		repo.CertData = certData
		repo.CertFile = "" // Clear file path if data is provided
	}

	if keyData, ok := secret.Data["tls.key"]; ok && len(keyData) > 0 {
		repo.KeyData = keyData
		repo.KeyFile = "" // Clear file path if data is provided
	}

	if val, ok := secret.Data["insecure"]; ok && len(val) > 0 {
		insecure := string(val) == "true"
		repo.Insecure = &insecure
	}

	return &repo, nil
}

func (cr *ChartReconciler) updateOrInstallChart(ctx context.Context, chart helmv1beta1.Chart) (err error) {
	var chartRelease *release.Release
	var timeout time.Duration
	timeout, err = time.ParseDuration(chart.Spec.Timeout)
	if err != nil {
		timeout = defaultTimeout
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}
	defer func() {
		if err == nil {
			return
		}
		if statusErr := apiretry.RetryOnConflict(apiretry.DefaultRetry, func() error {
			return cr.updateStatus(ctx, chart, chartRelease, err)
		}); statusErr != nil {
			cr.L.WithError(statusErr).Error("Failed to update status for chart release, give up", chart.Name)
		}
	}()

	// Get repository configuration - prefer embedded Repository over cache lookup
	var repo *helm.Repository
	chartName := chart.Spec.ChartName

	if chart.Spec.Repository != nil {
		// Load and merge repository configuration from secret (if configured) and inline fields
		repo, err = cr.loadAndMergeRepositoryConfig(ctx, chart)
		if err != nil {
			err = fmt.Errorf("can't load repository config for chart %q: %w", chartName, err)
			return
		}

		// For OCI charts, if URL is not set, extract registry URL from chartName
		if strings.HasPrefix(chartName, "oci://") && repo.URL == "" {
			repo.URL = extractOCIRegistryURL(chartName)
		}
	} else {
		// Fall back to extracting repository from chart name and looking it up in cache
		repo, err = cr.extractAndLookupRepository(chartName)
		if err != nil {
			err = fmt.Errorf("can't lookup repository for chart %q: %w", chartName, err)
			return
		}
	}

	// Create ephemeral Helm commands with repository (helm manages tmpDir internally)
	var helmCmd *helm.Commands
	var cleanup func()
	helmCmd, cleanup, err = helm.NewCommands(cr.kubeConfig, repo)
	if err != nil {
		err = fmt.Errorf("can't create Helm commands for chart %q: %w", chart.GetName(), err)
		return
	}
	defer cleanup()

	if chart.Status.ReleaseName == "" {
		// new chartRelease
		chartRelease, err = helmCmd.InstallChart(ctx,
			chartName,
			chart.Spec.Version,
			chart.Spec.ReleaseName,
			chart.Spec.Namespace,
			chart.Spec.YamlValues(),
			timeout,
		)
		if err != nil {
			err = fmt.Errorf("can't reconcile installation for %q: %w", chart.GetName(), err)
			return
		}
	} else {
		if !cr.chartNeedsUpgrade(chart) {
			return nil
		}
		// update
		chartRelease, err = helmCmd.UpgradeChart(ctx,
			chartName,
			chart.Spec.Version,
			chart.Status.ReleaseName,
			chart.Status.Namespace,
			chart.Spec.YamlValues(),
			timeout,
			chart.Spec.ShouldForceUpgrade(),
		)
		if err != nil {
			err = fmt.Errorf("can't reconcile upgrade for %q: %w", chart.GetName(), err)
			return
		}
	}
	if statusErr := apiretry.RetryOnConflict(apiretry.DefaultRetry, func() error {
		return cr.updateStatus(ctx, chart, chartRelease, nil)
	}); statusErr != nil {
		cr.L.WithError(statusErr).Error("Failed to update status for chart release, give up", chart.Name)
	}
	return nil
}

// extractAndLookupRepository extracts the repository name or OCI hostname from chartName
// and looks it up in the repository cache. Returns a pointer to the repository config or nil
// if no repository configuration is needed (e.g., local path charts, OCI charts without auth).
func (cr *ChartReconciler) extractAndLookupRepository(chartName string) (*helm.Repository, error) {
	repoID := extractRepositoryIdentifier(chartName)

	if repoID == "" {
		// Local path or invalid format
		if filepath.IsAbs(chartName) || strings.HasPrefix(chartName, ".") {
			return nil, nil // Local chart, no repo needed
		}
		// Traditional chart without slash - error
		return nil, fmt.Errorf("invalid chart name %q: expected format 'repository/chart' for non-OCI charts", chartName)
	}

	// For OCI, missing repo is okay (anonymous access)
	if registry.IsOCI(chartName) {
		cachedRepo := cr.repositoryCache.get(repoID)
		if cachedRepo == nil {
			return nil, nil
		}
		helmRepo := cachedRepo.ToHelm()
		return &helmRepo, nil
	}

	// For traditional, missing repo is an error
	cachedRepo := cr.repositoryCache.get(repoID)
	if cachedRepo == nil {
		return nil, fmt.Errorf("repository '%s' not found in cluster configuration", repoID)
	}
	helmRepo := cachedRepo.ToHelm()
	return &helmRepo, nil
}

// extractRepositoryIdentifier extracts the repository identifier from a chart name.
// For OCI charts, returns the hostname. For traditional charts, returns the repo name.
//
// Examples:
//   - "oci://ghcr.io/org/chart" → "ghcr.io"
//   - "oci://registry:8080/chart" → "registry:8080"
//   - "myrepo/mychart" → "myrepo"
//   - "/path/to/chart.tgz" → ""
//   - "./chart" → ""
//
// Returns empty string for local paths or if no identifier can be extracted.
func extractRepositoryIdentifier(chartName string) string {
	// Local paths don't have repository identifiers
	if filepath.IsAbs(chartName) || strings.HasPrefix(chartName, ".") {
		return ""
	}

	// OCI charts: extract hostname from URL
	if registry.IsOCI(chartName) {
		if chartURL, err := url.Parse(chartName); err == nil {
			return chartURL.Host
		}
		return ""
	}

	// Traditional charts: extract repo name before first slash
	if repoName, _, found := strings.Cut(chartName, "/"); found {
		return repoName
	}

	return ""
}

// extractOCIRegistryURL extracts the base registry URL from an OCI chart name.
//
// Examples:
//   - "oci://ghcr.io/user/charts/mychart" → "oci://ghcr.io"
//   - "oci://registry:8080/chart" → "oci://registry:8080"
//   - "myrepo/chart" → "" (not OCI)
//
// Returns empty string for non-OCI charts.
func extractOCIRegistryURL(chartName string) string {
	if !strings.HasPrefix(chartName, "oci://") {
		return ""
	}

	chartURL, err := url.Parse(chartName)
	if err != nil || chartURL.Host == "" {
		return ""
	}

	return "oci://" + chartURL.Host
}

func (cr *ChartReconciler) chartNeedsUpgrade(chart helmv1beta1.Chart) bool {
	return chart.Status.Namespace != chart.Spec.Namespace ||
		chart.Status.ReleaseName != chart.Spec.ReleaseName ||
		chart.Status.Version != chart.Spec.Version ||
		chart.Status.ValuesHash != chart.Spec.HashValues()
}

// updateStatus updates the status of the chart with the given release information. This function
// starts by fetching an updated version of the chart from the api as the install may take a while
// to complete and the chart may have been updated in the meantime. If returns the error returned
// by the Update operation. Moreover, if the chart has indeed changed in the meantime we already
// have an event for it so we will see it again soon.
func (cr *ChartReconciler) updateStatus(ctx context.Context, chart helmv1beta1.Chart, chartRelease *release.Release, err error) error {
	nsn := types.NamespacedName{Namespace: chart.Namespace, Name: chart.Name}
	var updchart helmv1beta1.Chart
	if err := cr.Get(ctx, nsn, &updchart); err != nil {
		return fmt.Errorf("can't get updated version of chart %s: %w", chart.Name, err)
	}
	chart.Spec.YamlValues() // XXX what is this function for ?
	if chartRelease != nil {
		updchart.Status.ReleaseName = chartRelease.Name
		updchart.Status.Version = chartRelease.Chart.Metadata.Version
		updchart.Status.AppVersion = chartRelease.Chart.AppVersion()
		updchart.Status.Revision = int64(chartRelease.Version)
		updchart.Status.Namespace = chartRelease.Namespace
	}
	updchart.Status.Updated = time.Now().String()
	updchart.Status.Error = ""
	if err != nil {
		updchart.Status.Error = err.Error()
	}
	updchart.Status.ValuesHash = chart.Spec.HashValues()
	if updErr := cr.Client.Status().Update(ctx, &updchart); updErr != nil {
		cr.L.WithError(updErr).Error("Failed to update status for chart release", chart.Name)
		return updErr
	}
	return nil
}

const chartCrdTemplate = `
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-{{ .Name }}
  namespace: ` + metav1.NamespaceSystem + `
  finalizers:
    - {{ .Finalizer }}
spec:
  chartName: {{ .ChartName }}
  releaseName: {{ .Name }}
  timeout: {{ .Timeout.Duration }}
  values: |
{{ .Values | nindent 4 }}
  version: {{ .Version }}
  namespace: {{ .TargetNS }}
{{- if ne .ForceUpgrade nil }}
  forceUpgrade: {{ .ForceUpgrade }}
{{- end }}

{{- if .Repository }}
  repository:
{{- if .Repository.URL }}
    url: {{ .Repository.URL }}
{{- end }}
{{- if .Repository.Username }}
    username: {{ .Repository.Username }}
{{- end }}
{{- if .Repository.Password }}
    password: {{ .Repository.Password }}
{{- end }}
{{- if .Repository.CAFile }}
    caFile: {{ .Repository.CAFile }}
{{- end }}
{{- if .Repository.CertFile }}
    certFile: {{ .Repository.CertFile }}
{{- end }}
{{- if .Repository.KeyFile }}
    keyFile: {{ .Repository.KeyFile }}
{{- end }}
{{- if ne .Repository.Insecure nil }}
    insecure: {{ .Repository.Insecure }}
{{- end }}
{{- end }}
`

const finalizerName = "helm.k0sproject.io/uninstall-helm-release"

// Init
func (ec *ExtensionsController) Init(_ context.Context) error {
	return nil
}

// Start
func (ec *ExtensionsController) Start(ctx context.Context) error {
	ec.L.Debug("Starting")

	mgr, err := ec.instantiateManager(ctx)
	if err != nil {
		return fmt.Errorf("can't instantiate controller-runtime manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		leaderelection.RunLeaderTasks(ctx, ec.leaderElector.CurrentStatus, func(ctx context.Context) {
			ec.L.Info("Running controller-runtime manager")
			if err := mgr.Start(ctx); err != nil {
				ec.L.WithError(err).Error("Failed to run controller-runtime manager")
			}
			ec.L.Info("Controller-runtime manager exited")
		})

	}()

	ec.stop = func() {
		cancel()
		<-done
	}

	return nil
}

func (ec *ExtensionsController) instantiateManager(ctx context.Context) (crman.Manager, error) {
	clientConfig, err := clientcmd.BuildConfigFromFlags("", ec.kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	gk := schema.GroupKind{
		Group: helmv1beta1.GroupName,
		Kind:  "Chart",
	}

	// Create a scheme with both k0s types and core Kubernetes types (needed for Secret access)
	scheme := k0sscheme.Scheme
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("can't add corev1 to scheme: %w", err)
	}

	mgr, err := controllerruntime.NewManager(clientConfig, crman.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Logger:     logrusr.New(ec.L),
		Controller: ctrlconfig.Controller{},
	})
	if err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	if err := retry.Do(func() error {
		_, err := mgr.GetRESTMapper().RESTMapping(gk)
		if err != nil {
			ec.L.Warn("Extensions CRD is not yet ready, waiting before starting ExtensionsController")
			return err
		}
		ec.L.Info("Extensions CRD is ready, going nuts")
		return nil
	}, retry.Context(ctx)); err != nil {
		return nil, fmt.Errorf("can't start ExtensionsReconciler, helm CRD is not registered, check CRD registration reconciler: %w", err)
	}

	if err := builder.
		ControllerManagedBy(mgr).
		Named("chart").
		For(&helmv1beta1.Chart{},
			builder.WithPredicates(predicate.And(
				predicate.GenerationChangedPredicate{},
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					return object.GetNamespace() == namespaceToWatch
				}),
			),
			),
		).
		Complete(&ChartReconciler{
			Client:          mgr.GetClient(),
			kubeConfig:      ec.kubeConfig,
			repositoryCache: ec.repositoryCache,
			leaderElector:   ec.leaderElector, // TODO: drop in favor of controller-runtime lease manager?
			L:               ec.L.WithField("extensions_type", "helm"),
		}); err != nil {
		return nil, fmt.Errorf("can't build controller-runtime controller for helm extensions: %w", err)
	}
	return mgr, nil
}

// Stop
func (ec *ExtensionsController) Stop() error {
	if ec.stop != nil {
		ec.stop()
	}
	ec.L.Debug("Stopped extensions controller")
	return nil
}
