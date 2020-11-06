package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func stringptr(s string) *string {
	return &s
}

func downloadChart(repository string, chartName string) (string, error) {
	//returns tmp directory chart, error
	return chartName, nil
}

func checkDeps(chartRequested *chart.Chart, cp string, client *action.Install) {
	//if req := chartRequested.Metadata.Dependencies; req != nil {
	//	// If CheckDependencies returns an error, we have unfulfilled dependencies.
	//	// As of Helm 2.4.0, this is treated as a stopping condition:
	//	// https://github.com/helm/helm/issues/2209
	//	if err := action.CheckDependencies(chartRequested, req); err != nil {
	//		man := &downloader.Manager{
	//			Out:              os.Stdout,
	//			ChartPath:        cp,
	//			Keyring:          client.ChartPathOptions.Keyring,
	//			SkipUpdate:       false,
	//			Getters:          p,
	//			RepositoryConfig: settings.RepositoryConfig,
	//			RepositoryCache:  settings.RepositoryCache,
	//			Debug:            settings.Debug,
	//		}
	//		if err := man.Update(); err != nil {
	//			return nil, err
	//		}
	//		// Reload the chart with the updated Chart.lock file.
	//		if chartRequested, err = loader.Load(cp); err != nil {
	//			return nil, errors.Wrap(err, "failed reloading chart after repo update")
	//		}
	//
	//	}
	//}
}

func install(repository string, chartName string, values map[string]interface{}, cfg *action.Configuration) (string, error) {
	chartDir, err := downloadChart(repository, chartName)

	install := action.NewInstall(cfg)
	install.Namespace = "kube-system"
	install.GenerateName = true
	name, _, err := install.NameAndChart([]string{chartName})
	install.ReleaseName = name

	if err != nil {
		return "", err
	}
	chart, err := loader.Load(chartDir)
	if err != nil {
		return "", fmt.Errorf("can't load chart `%s`: %v", chartDir, err)
	}
	if chart.Metadata.Type != "application" {
		return "", fmt.Errorf("chart with type `%s` is not installable", chart.Metadata.Type)
	}

	release, err := install.Run(chart, values)
	if err != nil {
		return "", fmt.Errorf("can't install chart `%s`: %v", chart.Name(), err)
	}
	return release.Name, nil
}

func upgrade(repository string, chartPath string, releaseName string, values map[string]interface{}, cfg *action.Configuration) (string, error) {
	chartDir, err := downloadChart(repository, chartPath)

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = "kube-system"

	chart, err := loader.Load(chartDir)
	if err != nil {
		return "", fmt.Errorf("can't load chart `%s`: %v", chartDir, err)
	}
	if chart.Metadata.Type != "application" {
		return "", fmt.Errorf("chart with type `%s` is not installable", chart.Metadata.Type)
	}

	release, err := upgrade.Run(releaseName, chart, values)
	spew.Dump(release, err)
	if err != nil {
		return "", fmt.Errorf("can't upgrade chart `%s`: %v", chart.Metadata.Name, err)
	}

	return release.Name, nil
}

func main() {
	chart := "/home/vagrant/mychart"

	fmt.Println("installing chart", chart)
	insecure := false
	impersonateGroup := []string{}
	cfg := &genericclioptions.ConfigFlags{
		Insecure:   &insecure,
		Timeout:    stringptr("0"),
		KubeConfig: stringptr(""),

		CacheDir:         stringptr("/var/lib/mke/cache"),
		ClusterName:      stringptr(""),
		AuthInfoName:     stringptr(""),
		Context:          stringptr(""),
		Namespace:        stringptr("kube-system"),
		APIServer:        stringptr(""),
		TLSServerName:    stringptr(""),
		CertFile:         stringptr(""),
		KeyFile:          stringptr(""),
		CAFile:           stringptr(""),
		BearerToken:      stringptr(""),
		Impersonate:      stringptr(""),
		ImpersonateGroup: &impersonateGroup,
	}
	actionConfig := &action.Configuration{}
	actionConfig.Init(cfg, "kube-system", "secret", func(format string, v ...interface{}) {
		fmt.Printf(format, v...)
	})
	spew.Dump(install("", chart, map[string]interface{}{}, actionConfig))

	//spew.Dump(upgrade("repo", chart, "mychart-1604490961", map[string]interface{}{
	//	"autoscaling": map[string]interface{}{"enabled": false},
	//}, actionConfig))
}
