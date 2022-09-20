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

	if err := tf.Init(ctx, tfexec.BackendConfig(fmt.Sprintf("path=%s", constant.TerraformStateFile))); err != nil {
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

	if err := tf.Init(ctx, tfexec.BackendConfig(fmt.Sprintf("path=%s", constant.TerraformStateFile))); err != nil {
		return fmt.Errorf("unable to init terraform in '%s': %w", workDir, err)
	}

	return handler(ctx, tf, opts...)
}
