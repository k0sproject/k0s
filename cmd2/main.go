package main

import (
	"github.com/Mirantis/mke/pkg/helm"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	//chart := "stable/nginx-ingress"
	//version := "1.41.3"
	//values := map[string]interface{}{
	//
	//}
	cmd := helm.NewCommands()

	cmd.ListReleases("default")
	//releaseName, err := cmd.InstallChart(chart, version, "default", values)
	//releaseName, err := cmd.UpgradeChart(chart, version, "nginx-ingress-1604754630", "default", values)
	//check(err)
	//fmt.Printf("Release %s installed", releaseName)

}
