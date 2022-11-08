// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/cavaliergopher/grab/v3"
	ts "github.com/k0sproject/k0s/inttest/toolsuite"
)

const (
	sonobuoyVersion = "0.56.11"
	sonobuoyOs      = "linux"
	sonobuoyArch    = "amd64"
)

type SonobuoyConfig struct {
	Parameters []string
}

// SonobuoyOperation builds a ClusterOperation that runs a Sonobuoy k8s conformance test
// using the clusters k0s.kubeconfig. Results are stored in the data directory as `results.tar.gz`
func SonobuoyOperation(config SonobuoyConfig) ts.ClusterOperation {
	return func(ctx context.Context, data ts.ClusterData) error {
		sonobuoyBinary, cleanup, err := downloadSonobuoy(ctx, sonobuoyVersion, sonobuoyOs, sonobuoyArch)
		defer cleanup()

		if err != nil {
			return fmt.Errorf("failed to download sonobuoy distribution")
		}

		// Run sonobuoy, and wait

		runArgs := []string{"run", "--kubeconfig", data.KubeConfigFile, "--wait"}
		runArgs = append(runArgs, config.Parameters...)

		sonobuoyCommand := exec.Command(sonobuoyBinary, runArgs...)
		sonobuoyCommand.Stdout = os.Stdout
		sonobuoyCommand.Stderr = os.Stderr
		err = sonobuoyCommand.Run()

		if err != nil {
			fmt.Printf("Received an error running sonobuoy, will attempt to collect results: %v\n", err)
		}

		// Collect the results

		resultsCommand := exec.Command(sonobuoyBinary, "retrieve", data.DataDir, "--kubeconfig", data.KubeConfigFile, "--filename", "results.tar.gz")
		resultsCommand.Stdout = os.Stdout
		resultsCommand.Stderr = os.Stderr
		return resultsCommand.Run()
	}
}

// downloadSonobuoy downloads the sonobuoy distribution for the provided OS and architecture.
// The result is a path to the sonobuoy binary in a temporary directory, a cleanup function to remove
// the transient binary/downloads, and any error that occurs.
func downloadSonobuoy(ctx context.Context, version string, osname string, arch string) (string, func(), error) {
	// As we create files, add them for removal
	var filesToRemove []string
	cleanup := func() {
		for _, f := range filesToRemove {
			_ = os.RemoveAll(f)
		}
	}

	sonobuoyUrl := fmt.Sprintf("https://github.com/vmware-tanzu/sonobuoy/releases/download/v%s/sonobuoy_%s_%s_%s.tar.gz", version, version, osname, arch)
	resp, err := grab.Get("/tmp", sonobuoyUrl)
	if err != nil {
		return "", cleanup, fmt.Errorf("failed to download the sonobuoy distribution: %w", err)
	}

	filesToRemove = append(filesToRemove, resp.Filename)

	// Extract the distribution to its own unique directory

	sonobuoyDir, err := os.MkdirTemp("/tmp", "sonobuoyop")
	if err != nil {
		return "", cleanup, err
	}

	filesToRemove = append(filesToRemove, sonobuoyDir)

	extractCommand := exec.Command("tar", "-C", sonobuoyDir, "-xzf", resp.Filename)
	extractCommand.Stdout = os.Stdout
	extractCommand.Stderr = os.Stderr

	if err := extractCommand.Run(); err != nil {
		return "", cleanup, fmt.Errorf("failed to extract sonobuoy distribution: %w", err)
	}

	return path.Join(sonobuoyDir, "sonobuoy"), cleanup, nil
}
