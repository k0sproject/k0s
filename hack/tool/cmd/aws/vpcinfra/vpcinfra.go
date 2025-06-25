// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package vpcinfra

import (
	"context"
	"fmt"

	"tool/pkg/backend/aws"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	opts options
)

func NewCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "vpcinfra",
		Short: "AWS VPC-based infrastructure commands",
	}

	cmd.AddCommand(newCommandCreate())
	cmd.AddCommand(newCommandDestroy())

	return &cmd
}

type options struct {
	Name   string
	Region string
	Cidr   string
}

func newOptionsFlagSet(f *options) *pflag.FlagSet {
	fs := pflag.FlagSet{}

	fs.StringVar(&f.Name, "name", "", "The name of the VPC infrastructure")
	fs.StringVar(&f.Region, "region", "", "The region to create in")
	fs.StringVar(&f.Cidr, "cidr", "", "The CIDR block for the VPC")

	return &fs
}

// buildCommand creates common cobra.Command instances that are only different in their
// execution function to allow for symmetric flags across `RunE` implementations.
func buildCommand(name, desc string, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	cmd := &cobra.Command{Use: name, Short: desc, RunE: runE}
	cmd.Flags().AddFlagSet(newOptionsFlagSet(&opts))

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("cidr")

	return cmd
}

// newCommandCreate creates a cobra.Command for creating a VPC infrastructure for a k0s cluster.
func newCommandCreate() *cobra.Command {
	return buildCommand(
		"create",
		"Create an AWS VPC infrastructure",
		func(cmd *cobra.Command, args []string) error {
			provider := aws.Provider{}

			ctx := context.Background()
			if err := provider.Init(ctx); err != nil {
				return fmt.Errorf("failed to initialize AWS provider: %w", err)
			}

			if err := provider.VpcInfraCreate(ctx, opts.Name, opts.Region, opts.Cidr); err != nil {
				return fmt.Errorf("failed to create VPC infrastructure: %w", err)
			}

			return nil
		},
	)
}

// newCommandCreate creates a cobra.Command for destroying a VPC infrastructure for a k0s cluster.
func newCommandDestroy() *cobra.Command {
	return buildCommand(
		"destroy",
		"Destroy an AWS VPC infrastructure",
		func(cmd *cobra.Command, args []string) error {
			provider := aws.Provider{}

			ctx := context.Background()
			if err := provider.Init(ctx); err != nil {
				return fmt.Errorf("failed to initialize AWS provider: %w", err)
			}

			if err := provider.VpcInfraDestroy(ctx, opts.Name, opts.Region, opts.Cidr); err != nil {
				return fmt.Errorf("failed to destroy VPC infrastructure: %w", err)
			}

			return nil
		},
	)
}
