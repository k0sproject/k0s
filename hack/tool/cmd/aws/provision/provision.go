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
package provision

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"tool/pkg/constant"

	k0sctlcmd "github.com/k0sproject/k0sctl/cmd"
)

type ProvisionConfig struct {
	Init          func(ctx context.Context) error
	Create        func(ctx context.Context) error
	ClusterConfig func(ctx context.Context) (string, error)
}

func Provision(ctx context.Context, config ProvisionConfig) error {
	if err := config.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}

	if err := config.Create(ctx); err != nil {
		return fmt.Errorf("failed to create k0s cluster infrastructure: %w", err)
	}

	clusterConfig, err := config.ClusterConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch cluster config: %w", err)
	}

	// Invoke k0sctl

	f, err := os.Create(constant.DataDirK0sctlConfig)
	if err != nil {
		return fmt.Errorf("failed to create '%s': %w", constant.DataDirK0sctlConfig, err)
	}

	if _, err := f.WriteString(clusterConfig); err != nil {
		return fmt.Errorf("failed to write to '%s': %w", constant.DataDirK0sctlConfig, err)
	}

	// Post-processing of k0sctl yaml
	bugfixK0sctlNullImages(constant.DataDirK0sctlConfig)

	if err := k0sctlcmd.App.Run([]string{"k0sctl", "apply", "-c", constant.DataDirK0sctlConfig}); err != nil {
		return fmt.Errorf("failed to create k0s cluster with k0sctl: %w", err)
	}

	// Generate the kubeconfig

	kubeconfig, err := os.Create(constant.DataDirK0sKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create '%s': %w", constant.DataDirK0sKubeConfig, err)
	}

	defer kubeconfig.Close()

	k0sctlcmd.App.Writer = kubeconfig

	if err := k0sctlcmd.App.Run([]string{"k0sctl", "kubeconfig", "-c", constant.DataDirK0sctlConfig}); err != nil {
		return fmt.Errorf("failed to extract kubeconfig using k0sctl: %w", err)
	}

	return nil
}

type DeprovisionConfig struct {
	Init    func(ctx context.Context) error
	Destroy func(ctx context.Context) error
}

func Deprovision(ctx context.Context, config DeprovisionConfig) error {
	if err := config.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}

	return config.Destroy(ctx)
}

func bugfixK0sctlNullImages(k0sctlConfigFile string) error {
	cmd := exec.Command("sed", "-i", "/images.*null/d", k0sctlConfigFile)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to post-process k0sctl.yaml")
	}

	return nil
}
