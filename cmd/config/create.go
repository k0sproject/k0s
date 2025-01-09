/*
Copyright 2021 k0s authors

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

package config

import (
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

func NewCreateCmd() *cobra.Command {
	var includeImages bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Output the default k0s configuration yaml to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config := v1beta1.DefaultClusterConfig()
			if !includeImages {
				config.Spec.Images = nil
				config.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image = nil
			}

			var u unstructured.Unstructured
			if err := k0sscheme.Scheme.Convert(config, &u, nil); err != nil {
				return err
			}
			unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")

			cfg, err := yaml.Marshal(&u)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(cfg)
			return err
		},
	}

	flags := cmd.Flags()
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
		cmd.PersistentFlags().AddFlag(f)
	})
	flags.BoolVar(&includeImages, "include-images", false, "include the default images in the output")

	return cmd
}
