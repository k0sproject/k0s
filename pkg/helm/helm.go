// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Repository represents a Helm chart repository configuration.
type Repository struct {
	Name     string
	URL      string
	Username string
	Password string
	// File paths for certificates (used when certs are already on disk)
	CAFile   string
	CertFile string
	KeyFile  string
	// In-memory certificate data (used when certs come from secrets)
	// These take precedence over file paths when both are set
	CAData   []byte
	CertData []byte
	KeyData  []byte
	Insecure *bool
}

// IsInsecure returns true if TLS verification should be skipped.
func (r *Repository) IsInsecure() bool {
	return r.Insecure != nil && *r.Insecure
}

// Commands run different helm command in the same way as CLI tool
// This struct isn't thread-safe. Check on a per function basis.
type Commands struct {
	registryManager *ociRegistryManager

	repoFile     string
	helmCacheDir string
	kubeConfig   string
}

func logFn(format string, args ...any) {
	log := logrus.WithField("component", "helm")
	log.Debugf(format, args...)
}

var getters = getter.Providers{
	getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	},
	getter.Provider{
		Schemes: []string{"oci"},
		New:     getter.NewOCIGetter,
	},
}

// NewCommands builds new Commands instance with ephemeral temporary directories.
// If repo is provided, it will be initialized in the temporary environment.
// If tmpDir is empty, a new temporary directory will be created.
// Returns the Commands instance and a cleanup function that must be called to remove temporary files.
func NewCommands(kubeConfig string, repo *Repository) (*Commands, func(), error) {
	// Create temporary directory for ephemeral Helm environment
	var err error
	tmpDir, err := os.MkdirTemp("", "k0s-helm-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary Helm directory: %w", err)
	}

	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			logrus.WithError(err).WithField("path", tmpDir).Warn("Failed to clean up temporary Helm directory")
		}
	}

	repoFile := filepath.Join(tmpDir, "repositories.yaml")
	helmCacheDir := filepath.Join(tmpDir, "cache")

	commands := &Commands{
		repoFile:        repoFile,
		registryManager: newOCIRegistryManager(),
		helmCacheDir:    helmCacheDir,
		kubeConfig:      kubeConfig,
	}

	// Initialize repository if provided
	if repo != nil {
		// Write in-memory cert data to temp files if needed for traditional repos
		if err := commands.prepareCertFiles(repo, tmpDir); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to prepare certificate files: %w", err)
		}

		if err := commands.initRepository(*repo); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to initialize repository: %w", err)
		}
	}

	return commands, cleanup, nil
}

// prepareCertFiles writes in-memory certificate data to temp files when needed.
// For OCI registries, we can use in-memory certs directly, so this is only needed
// for traditional repos that require file paths.
func (hc *Commands) prepareCertFiles(repo *Repository, tmpDir string) error {
	// Skip if using OCI (we can handle certs in-memory for OCI)
	if registry.IsOCI(repo.URL) {
		return nil
	}

	// Write CA data to file if present
	if len(repo.CAData) > 0 && repo.CAFile == "" {
		caPath := filepath.Join(tmpDir, "ca.crt")
		if err := os.WriteFile(caPath, repo.CAData, 0600); err != nil {
			return fmt.Errorf("failed to write CA data to temp file: %w", err)
		}
		repo.CAFile = caPath
	}

	// Write cert data to file if present
	if len(repo.CertData) > 0 && repo.CertFile == "" {
		certPath := filepath.Join(tmpDir, "tls.crt")
		if err := os.WriteFile(certPath, repo.CertData, 0600); err != nil {
			return fmt.Errorf("failed to write cert temp file: %w", err)
		}
		repo.CertFile = certPath
	}

	// Write key data to file if present
	if len(repo.KeyData) > 0 && repo.KeyFile == "" {
		keyPath := filepath.Join(tmpDir, "tls.key")
		if err := os.WriteFile(keyPath, repo.KeyData, 0600); err != nil {
			return fmt.Errorf("failed to write key data to temp file: %w", err)
		}
		repo.KeyFile = keyPath
	}

	return nil
}

func (hc *Commands) getActionCfg(namespace string) (*action.Configuration, error) {
	// Construct new helm env so we get the retrying roundtripper etc. setup
	// See https://github.com/helm/helm/pull/11426/commits/b5378b3a5dd435e5c364ac0cfa717112ad686bd0
	helmEnv := cli.New()
	helmFlags, ok := helmEnv.RESTClientGetter().(*genericclioptions.ConfigFlags)
	if !ok {
		return nil, errors.New("failed to construct Helm REST client")
	}

	insecure := false
	var impersonateGroup []string
	cfg := &genericclioptions.ConfigFlags{
		Insecure:         &insecure,
		Timeout:          ptr.To("0"),
		KubeConfig:       ptr.To(hc.kubeConfig),
		CacheDir:         ptr.To(hc.helmCacheDir),
		Namespace:        ptr.To(namespace),
		ImpersonateGroup: &impersonateGroup,
		WrapConfigFn:     helmFlags.WrapConfigFn, // This contains the retrying round tripper
	}
	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(cfg, namespace, "secret", logFn); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// initRepository initializes a single repository in the ephemeral Helm environment
func (hc *Commands) initRepository(repoCfg Repository) error {
	if err := hc.registryManager.AddRegistry(repoCfg); !errors.Is(err, errors.ErrUnsupported) {
		return err
	}

	err := dir.Init(filepath.Dir(hc.repoFile), constant.DataDirMode)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("can't add repository to %s: %w", hc.repoFile, err)
	}

	b, err := os.ReadFile(hc.repoFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("can't add repository to %s: %w", hc.repoFile, err)
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("can't add repository to %s: %w", hc.repoFile, err)
	}

	c := repo.Entry{
		Name:                  repoCfg.Name,
		URL:                   repoCfg.URL,
		Username:              repoCfg.Username,
		Password:              repoCfg.Password,
		CertFile:              repoCfg.CertFile,
		KeyFile:               repoCfg.KeyFile,
		CAFile:                repoCfg.CAFile,
		InsecureSkipTLSverify: repoCfg.IsInsecure(),
	}

	r, err := repo.NewChartRepository(&c, getters)
	if err != nil {
		return fmt.Errorf("can't add repository to %s: %w", hc.repoFile, err)
	}
	r.CachePath = hc.helmCacheDir

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("can't add repository: %q is not a valid chart repository or cannot be reached: %w", "repo", err)
	}
	f.Update(&c)
	if err := f.WriteFile(hc.repoFile, 0644); err != nil {
		return fmt.Errorf("can't add repository to %s: %w", hc.repoFile, err)
	}

	return nil
}

func (hc *Commands) downloadDependencies(chart *chart.Chart, chartPath string, registryClient *registry.Client) error {
	if chart.Metadata.Dependencies == nil {
		return nil
	}
	if err := action.CheckDependencies(chart, chart.Metadata.Dependencies); err != nil {
		man := &downloader.Manager{
			Out:              os.Stdout,
			ChartPath:        chartPath,
			SkipUpdate:       false,
			Getters:          getters,
			RepositoryConfig: hc.repoFile,
			RepositoryCache:  hc.helmCacheDir,
			Debug:            false,
			RegistryClient:   registryClient,
		}
		if err := man.Update(); err != nil {
			return err
		}
	}
	return nil
}

func (hc *Commands) locateChart(name string, version string, registryClient *registry.Client) (string, error) {
	name = strings.TrimSpace(name)

	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, fmt.Errorf("can't locate chart `%s-%s`: %w", name, version, err)
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("can't locate chart: path not found: %s", name)
	}

	dl := downloader.ChartDownloader{
		Out:              os.Stdout,
		Getters:          getters,
		Options:          []getter.Option{getter.WithRegistryClient(registryClient)},
		RepositoryConfig: hc.repoFile,
		RepositoryCache:  hc.helmCacheDir,
		RegistryClient:   registryClient,
	}

	if err := dir.Init(hc.helmCacheDir, constant.DataDirMode); err != nil {
		return "", fmt.Errorf("can't locate chart `%s-%s`: %w", name, version, err)
	}

	filename, _, err := dl.DownloadTo(name, version, hc.helmCacheDir)
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, fmt.Errorf("can't locate chart `%s-%s`: %w", name, version, err)
		}
		return lname, nil
	}
	return filename, fmt.Errorf("can't locate chart `%s-%s`: %w", name, version, err)
}

func (hc *Commands) isInstallable(chart *chart.Chart) bool {
	if chart.Metadata.Type != "" && chart.Metadata.Type != "application" {
		return false
	}
	return true
}

// InstallChart installs a helm chart
// InstallChart, UpgradeChart and UninstallRelease(releaseName are *NOT* thread-safe
func (hc *Commands) InstallChart(ctx context.Context, chartName string, version string, releaseName string, namespace string, values map[string]any, timeout time.Duration) (*release.Release, error) {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create action configuration: %w", err)
	}

	cfg.RegistryClient, err = hc.registryManager.GetRegistryClient(chartName)
	if err != nil {
		return nil, fmt.Errorf("can't get registry client for chart `%s`: %w", chartName, err)
	}

	install := action.NewInstall(cfg)
	install.CreateNamespace = true
	install.WaitForJobs = true
	install.Wait = true
	install.Timeout = timeout
	chartDir, err := hc.locateChart(chartName, version, cfg.RegistryClient)
	if err != nil {
		return nil, err
	}
	install.Namespace = namespace
	install.Atomic = true
	install.ReleaseName = releaseName
	name, _, err := install.NameAndChart([]string{chartName})
	install.ReleaseName = name

	if err != nil {
		return nil, err
	}

	loadedChart, err := loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("can't load loadedChart `%s`: %w", chartDir, err)
	}
	if !hc.isInstallable(loadedChart) {
		return nil, fmt.Errorf("loadedChart with type `%s` is not installable", loadedChart.Metadata.Type)
	}

	if err := hc.downloadDependencies(loadedChart, chartDir, cfg.RegistryClient); err != nil {
		return nil, err
	}

	loadedChart, err = loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("can't reload loadedChart `%s`: %w", chartDir, err)
	}
	chartRelease, err := install.RunWithContext(ctx, loadedChart, values)
	if err != nil {
		return nil, fmt.Errorf("can't install loadedChart `%s`: %w", loadedChart.Name(), err)
	}

	return chartRelease, nil
}

// UpgradeChart upgrades a helm chart.
// InstallChart, UpgradeChart and UninstallRelease(releaseName are *NOT* thread-safe
func (hc *Commands) UpgradeChart(ctx context.Context, chartName string, version string, releaseName string, namespace string, values map[string]any, timeout time.Duration, force bool) (*release.Release, error) {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create action configuration: %w", err)
	}

	cfg.RegistryClient, err = hc.registryManager.GetRegistryClient(chartName)
	if err != nil {
		return nil, fmt.Errorf("can't get registry client for chart `%s`: %w", chartName, err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Wait = true
	upgrade.WaitForJobs = true
	upgrade.Install = true
	upgrade.Force = force
	upgrade.Atomic = true
	upgrade.Timeout = timeout
	chartDir, err := hc.locateChart(chartName, version, cfg.RegistryClient)
	if err != nil {
		return nil, err
	}
	loadedChart, err := loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("can't load loadedChart `%s`: %w", chartDir, err)
	}
	if !hc.isInstallable(loadedChart) {
		return nil, fmt.Errorf("loadedChart with type `%s` is not installable", loadedChart.Metadata.Type)
	}

	if err := hc.downloadDependencies(loadedChart, chartDir, cfg.RegistryClient); err != nil {
		return nil, err
	}

	loadedChart, err = loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("can't reload loadedChart `%s`: %w", chartDir, err)
	}

	chartRelease, err := upgrade.RunWithContext(ctx, releaseName, loadedChart, values)
	if err != nil {
		return nil, fmt.Errorf("can't upgrade loadedChart `%s`: %w", loadedChart.Metadata.Name, err)
	}

	return chartRelease, nil
}

func (hc *Commands) ListReleases(namespace string) ([]*release.Release, error) {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create helmAction configuration: %w", err)
	}
	helmAction := action.NewList(cfg)
	helmAction.StateMask = action.ListAll // Include all release states: deployed, failed, pending-install, pending-upgrade, pending-rollback, uninstalling, superseded
	return helmAction.Run()
}

// GetReleaseStatus returns the status of a specific Helm release by searching
// through all releases in the namespace. We use ListReleases instead of Helm's
// native status check to ensure we can find releases in any state (not just deployed).
// This is primarily used by the cleanup logic to detect stuck releases.
func (hc *Commands) GetReleaseStatus(releaseName string, namespace string) (release.Status, error) {
	releases, err := hc.ListReleases(namespace)
	if err != nil {
		return "", fmt.Errorf("can't list releases: %w", err)
	}
	for _, rel := range releases {
		if rel.Name == releaseName {
			return rel.Info.Status, nil
		}
	}
	return "", fmt.Errorf("release %q not found in namespace %q", releaseName, namespace)
}

// UninstallRelease uninstalls a release.
// The disableHooks parameter should be true when completing interrupted operations
// where hook resources may already exist, preventing "already exists" errors.
// InstallChart, UpgradeChart and UninstallRelease are *NOT* thread-safe
func (hc *Commands) UninstallRelease(ctx context.Context, releaseName string, namespace string, disableHooks bool) error {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return fmt.Errorf("can't create helmAction configuration: %w", err)
	}
	helmAction := action.NewUninstall(cfg)
	helmAction.DisableHooks = disableHooks
	helmAction.KeepHistory = false // Always purge release history completely to allow fresh installs
	deadline, ok := ctx.Deadline()
	if ok {
		helmAction.Timeout = time.Until(deadline)
	}

	if _, err := helmAction.Run(releaseName); err != nil {
		return fmt.Errorf("can't uninstall release `%s`: %w", releaseName, err)
	}
	return nil
}

// RollbackRelease rolls back a release to the previous deployed revision.
// This is used to recover from interrupted upgrades where the release is stuck in
// pending-upgrade or pending-rollback state. By rolling back, we preserve the
// previously deployed workloads instead of uninstalling everything.
// InstallChart, UpgradeChart, UninstallRelease and RollbackRelease are *NOT* thread-safe
func (hc *Commands) RollbackRelease(ctx context.Context, releaseName string, namespace string) error {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return fmt.Errorf("can't create helmAction configuration: %w", err)
	}
	helmAction := action.NewRollback(cfg)
	deadline, ok := ctx.Deadline()
	if ok {
		helmAction.Timeout = time.Until(deadline)
	}
	// Version 0 is Helm's convention for "rollback to the previous deployed revision"
	helmAction.Version = 0

	if err := helmAction.Run(releaseName); err != nil {
		return fmt.Errorf("can't rollback release `%s`: %w", releaseName, err)
	}
	return nil
}
