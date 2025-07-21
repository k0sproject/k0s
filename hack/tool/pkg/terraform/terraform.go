// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package terraform

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"tool/pkg/constant"

	"github.com/hashicorp/terraform-exec/tfexec"
)

func Apply(ctx context.Context, modulePath string, opts ...tfexec.ApplyOption) error {
	return execute(
		ctx,
		path.Join(constant.TerraformScriptDir, modulePath),
		func(ctx context.Context, tf *tfexec.Terraform, opts ...tfexec.ApplyOption) error {
			return tf.Apply(ctx, opts...)
		},
		opts...,
	)
}

func Destroy(ctx context.Context, modulePath string, opts ...tfexec.DestroyOption) error {
	return execute(
		ctx,
		path.Join(constant.TerraformScriptDir, modulePath),
		func(ctx context.Context, tf *tfexec.Terraform, opts ...tfexec.DestroyOption) error {
			return tf.Destroy(ctx, opts...)
		},
		opts...,
	)
}

func Output(ctx context.Context, modulePath string) (map[string]tfexec.OutputMeta, error) {
	scriptDir := path.Join(constant.TerraformScriptDir, modulePath)

	tf, err := tfexec.NewTerraform(scriptDir, constant.TerraformBinary)
	if err != nil {
		return nil, fmt.Errorf("unable to create 'terraform' instance: %w", err)
	}

	if err := tf.Init(ctx, tfexec.BackendConfig("path="+constant.TerraformStateFile)); err != nil {
		return nil, fmt.Errorf("unable to init terraform in '%s': %w", scriptDir, err)
	}

	return tf.Output(ctx)
}

type Handler[OT any] func(ctx context.Context, tf *tfexec.Terraform, opts ...OT) error

func execute[OT any](ctx context.Context, workDir string, handler Handler[OT], opts ...OT) error {
	tf, err := tfexec.NewTerraform(workDir, constant.TerraformBinary)
	if err != nil {
		return fmt.Errorf("unable to create 'terraform' instance: %w", err)
	}

	tf.SetLog("INFO")
	tf.SetStdout(os.Stdout)
	tf.SetLogger(log.Default())

	if err := tf.Init(ctx, tfexec.BackendConfig("path="+constant.TerraformStateFile)); err != nil {
		return fmt.Errorf("unable to init terraform in '%s': %w", workDir, err)
	}

	return handler(ctx, tf, opts...)
}
