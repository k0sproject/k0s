package version

import (
	"encoding/json"
	"fmt"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/spf13/cobra"
)

var (
	all   bool
	isJsn bool
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the k0s version",

		Run: func(cmd *cobra.Command, args []string) {
			info := versionInfo{
				Version:      build.Version,
				Runc:         build.RuncVersion,
				Containerd:   build.ContainerdVersion,
				Kubernetes:   build.KubernetesVersion,
				Kine:         build.KineVersion,
				Etcd:         build.EtcdVersion,
				Konnectivity: build.KonnectivityVersion,
			}

			info.String()
		},
	}

	// append flags
	cmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "use to print all k0s version info")
	cmd.PersistentFlags().BoolVarP(&isJsn, "json", "j", false, "use to print all k0s version info in json")
	return cmd
}

type versionInfo struct {
	Version      string `json:"k0s,omitempty"`
	Runc         string `json:"runc,omitempty"`
	Containerd   string `json:"containerd,omitempty"`
	Kubernetes   string `json:"kubernetes,omitempty"`
	Kine         string `json:"kine,omitempty"`
	Etcd         string `json:"etcd,omitempty"`
	Konnectivity string `json:"konnectivity,omitempty"`
}

func (v versionInfo) String() {
	if all {
		fmt.Println("k0s :", v.Version)
		fmt.Println("runc :", v.Runc)
		fmt.Println("containerd :", v.Containerd)
		fmt.Println("kubernetes :", v.Kubernetes)
		fmt.Println("kine :", v.Kine)
		fmt.Println("etcd :", v.Etcd)
		fmt.Println("konnectivity :", v.Konnectivity)
	} else if isJsn {
		jsn, _ := json.MarshalIndent(v, "", "   ")
		fmt.Println(string(jsn))
	} else {
		fmt.Println(v.Version)
	}
}
