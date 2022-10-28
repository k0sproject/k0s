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
package ha

import (
	"context"
	"fmt"
	"path"
	"tool/cmd/aws/provision"
	"tool/pkg/backend/aws"
	"tool/pkg/constant"
	"tool/pkg/k0s"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	opts options
)

type options struct {
	ClusterName           string
	Region                string
	Controllers           int
	Workers               int
	K0sBinary             string
	K0sVersion            string
	K0sAirgapBundle       string
	K0sAirgapBundleConfig string
	K0sUpdateBinary       string
	K0sUpdateVersion      string
	K0sUpdateAirgapBundle string
}

func NewCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ha",
		Short: "AWS HA cluster commands",
	}

	cmd.AddCommand(newCommandCreate())
	cmd.AddCommand(newCommandDestroy())

	return &cmd
}

func newOptionsFlagSet(f *options) *pflag.FlagSet {
	fs := pflag.FlagSet{}

	fs.StringVar(&f.ClusterName, "cluster-name", "", "The name of the k0s cluster")
	fs.StringVar(&f.Region, "region", "", "The AWS region to create resources in")

	fs.IntVar(&f.Controllers, "controllers", 1, "The number of k0s controllers to create")
	fs.IntVar(&f.Workers, "workers", 1, "The number of k0s workers to create")

	fs.StringVar(&f.K0sBinary, "k0s-binary", "", fmt.Sprintf("The k0s binary in '%s' that the cluster should be created with", constant.DataDir))
	fs.StringVar(&f.K0sVersion, "k0s-version", "", "The version of k0s to install")
	fs.StringVar(&f.K0sAirgapBundle, "k0s-airgap-bundle", "", "The k0s airgap bundle to install with k0s")
	fs.StringVar(&f.K0sAirgapBundleConfig, "k0s-airgap-bundle-config", "", fmt.Sprintf("A YAML definition in '%s' of all the airgap images + versions", constant.DataDir))
	fs.StringVar(&f.K0sUpdateVersion, "k0s-update-version", "", "The version of k0s that should be used for a cluster update")
	fs.StringVar(&f.K0sUpdateBinary, "k0s-update-binary", "", fmt.Sprintf("The k0s binary in '%s' that will be available for software update", constant.DataDir))
	fs.StringVar(&f.K0sUpdateAirgapBundle, "k0s-update-airgap-bundle", "", fmt.Sprintf("The k0s airgap bundle in '%s' that will be available for software update", constant.DataDir))

	return &fs
}

// buildCommand creates common cobra.Command instances that are only different in their
// execution function to allow for symmetric flags across `RunE` implementations.
func buildCommand(name, desc string, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	cmd := cobra.Command{Use: name, Short: desc, RunE: runE}
	cmd.Flags().AddFlagSet(newOptionsFlagSet(&opts))

	cmd.MarkFlagRequired("cluster-name")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("controllers")
	cmd.MarkFlagRequired("workers")

	cmd.MarkFlagsMutuallyExclusive("k0s-binary", "k0s-version")

	return &cmd
}

// newCommandCreate creates a cobra.Command for creating HA k0s clusters.
func newCommandCreate() *cobra.Command {
	return buildCommand(
		"create",
		"Create an HA k0s cluster",
		func(cmd *cobra.Command, args []string) error {
			provider := aws.Provider{}

			foundK0sVersion := opts.K0sVersion
			var err error

			provisionConfig := provision.ProvisionConfig{
				Init: func(ctx context.Context) error {
					if err := provider.Init(ctx); err != nil {
						return err
					}

					cmd.SilenceUsage = true

					// If a binary is specified, ensure it exists and extract the version information from it.
					// Otherwise, k0sctl will attempt to download the specified version.

					if opts.K0sBinary != "" {
						foundK0sVersion, err = k0s.Version(path.Join(constant.DataDir, opts.K0sBinary))
						if err != nil {
							return fmt.Errorf("unable to determine k0s version of '%s'", opts.K0sBinary)
						}
					}

					return nil
				},
				Create: func(ctx context.Context) error {
					// TODO: struct this, getting out of control
					return provider.ClusterHACreate(ctx, opts.ClusterName, opts.K0sBinary, opts.K0sUpdateBinary, foundK0sVersion, opts.K0sUpdateVersion, opts.K0sAirgapBundle, opts.K0sAirgapBundleConfig, opts.K0sUpdateAirgapBundle, opts.Controllers, opts.Workers, opts.Region)
				},
				ClusterConfig: func(ctx context.Context) (string, error) {
					return provider.ClusterHAClusterConfig(ctx)
				},
			}

			return provision.Provision(context.Background(), provisionConfig)
		},
	)
}

// newCommandDestroy creates a cobra.Command for destroying HA k0s clusters.
func newCommandDestroy() *cobra.Command {
	return buildCommand(
		"destroy",
		"Destroy an HA k0s cluster",
		func(cmd *cobra.Command, args []string) error {
			provider := aws.Provider{}

			config := provision.DeprovisionConfig{
				Init: func(ctx context.Context) error {
					if err := provider.Init(ctx); err != nil {
						return fmt.Errorf("failed to initialize AWS provider: %w", err)
					}

					cmd.SilenceUsage = true

					return nil
				},
				Destroy: func(ctx context.Context) error {
					// TODO: struct this, getting out of control
					return provider.ClusterHADestroy(ctx, opts.ClusterName, opts.K0sBinary, opts.K0sUpdateBinary, opts.K0sVersion, opts.K0sUpdateVersion, opts.K0sAirgapBundle, opts.K0sAirgapBundleConfig, opts.K0sUpdateAirgapBundle, opts.Controllers, opts.Workers, opts.Region)
				},
			}

			return provision.Deprovision(cmd.Context(), config)
		},
	)
}
