/*
Copyright 2022 k0s authors

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
