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
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Commands run different helm command in the same way as CLI tool
// This struct isn't thread-safe. Check on a per function basis.
type Commands struct {
	repoFile     string
	helmCacheDir string
	kubeConfig   string
}

func logFn(format string, args ...interface{}) {
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

// NewCommands builds new Commands instance with default values
func NewCommands(k0sVars *config.CfgVars) *Commands {
	return &Commands{
		repoFile:     k0sVars.HelmRepositoryConfig,
		helmCacheDir: k0sVars.HelmRepositoryCache,
		kubeConfig:   k0sVars.AdminKubeConfigPath,
	}
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

func (hc *Commands) AddRepository(repoCfg v1beta1.Repository) error {
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

func (hc *Commands) downloadDependencies(chart *chart.Chart, chartPath string) error {
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
		}
		if err := man.Update(); err != nil {
			return err
		}
	}
	return nil
}

func (hc *Commands) locateChart(name string, version string) (string, error) {
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
		Options:          []getter.Option{},
		RepositoryConfig: hc.repoFile,
		RepositoryCache:  hc.helmCacheDir,
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
func (hc *Commands) InstallChart(ctx context.Context, chartName string, version string, releaseName string, namespace string, values map[string]interface{}, timeout time.Duration) (*release.Release, error) {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create action configuration: %w", err)
	}
	install := action.NewInstall(cfg)
	install.CreateNamespace = true
	install.WaitForJobs = true
	install.Wait = true
	install.Timeout = timeout
	chartDir, err := hc.locateChart(chartName, version)
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

	if err := hc.downloadDependencies(loadedChart, chartDir); err != nil {
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
func (hc *Commands) UpgradeChart(ctx context.Context, chartName string, version string, releaseName string, namespace string, values map[string]interface{}, timeout time.Duration, force bool) (*release.Release, error) {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create action configuration: %w", err)
	}
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Wait = true
	upgrade.WaitForJobs = true
	upgrade.Install = true
	upgrade.Force = force
	upgrade.Atomic = true
	upgrade.Timeout = timeout
	chartDir, err := hc.locateChart(chartName, version)
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

	if err := hc.downloadDependencies(loadedChart, chartDir); err != nil {
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
	return helmAction.Run()
}

// UninstallRelease uninstalls a release.
// InstallChart, UpgradeChart and UninstallRelease(releaseName are *NOT* thread-safe
func (hc *Commands) UninstallRelease(ctx context.Context, releaseName string, namespace string) error {
	cfg, err := hc.getActionCfg(namespace)
	if err != nil {
		return fmt.Errorf("can't create helmAction configuration: %w", err)
	}
	helmAction := action.NewUninstall(cfg)
	deadline, ok := ctx.Deadline()
	if ok {
		helmAction.Timeout = time.Until(deadline)
	}

	if _, err := helmAction.Run(releaseName); err != nil {
		return fmt.Errorf("can't uninstall release `%s`: %w", releaseName, err)
	}
	return nil
}
