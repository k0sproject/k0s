// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

func NewCreateCmd() *cobra.Command {
	var (
		debugFlags    internal.DebugFlags
		includeImages bool
	)

	cmd := &cobra.Command{
		Use:              "create",
		Short:            "Output the default k0s configuration yaml to stdout",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
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

	pflags := cmd.PersistentFlags()
	debugFlags.AddToFlagSet(pflags)
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
		pflags.AddFlag(f)
	})

	cmd.Flags().BoolVar(&includeImages, "include-images", false, "include the default images in the output")

	return cmd
}
