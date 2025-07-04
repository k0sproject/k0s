// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"encoding/json"
	"fmt"
	"io"

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
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			info := versionInfo{
				Version:      build.Version,
				Runc:         build.RuncVersion,
				Containerd:   build.ContainerdVersion,
				Kubernetes:   build.KubernetesVersion,
				Kine:         build.KineVersion,
				Etcd:         build.EtcdVersion,
				Konnectivity: build.KonnectivityVersion,
			}

			info.Print(cmd.OutOrStdout())
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&all, "all", "a", false, "use to print all k0s version info")
	flags.BoolVarP(&isJsn, "json", "j", false, "use to print all k0s version info in json")

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

func (v versionInfo) Print(w io.Writer) {
	if all {
		fmt.Fprintln(w, "k0s :", v.Version)
		fmt.Fprintln(w, "runc :", v.Runc)
		fmt.Fprintln(w, "containerd :", v.Containerd)
		fmt.Fprintln(w, "kubernetes :", v.Kubernetes)
		fmt.Fprintln(w, "kine :", v.Kine)
		fmt.Fprintln(w, "etcd :", v.Etcd)
		fmt.Fprintln(w, "konnectivity :", v.Konnectivity)
	} else if isJsn {
		jsn, _ := json.MarshalIndent(v, "", "   ")
		fmt.Fprintln(w, string(jsn))
	} else {
		fmt.Fprintln(w, v.Version)
	}
}
