// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
)

// RequireContainerdV2ConfigSnippets registers a probe that checks every *.toml
// file found in importsDir for containerd configuration formatting issues such as wrong version or deprecated keys.
// Each incompatible file is reported individually as a rejection with
// remediation instructions.  If importsDir does not exist or contains no TOML
// files the top level probe passes.
func RequireContainerdV2ConfigSnippets(parent ParentProbe, importsDir string) {
	parent.Set("containerd:configSnippets", func(path ProbePath, _ Probe) Probe {
		return ProbeFn(func(r Reporter) error {
			groupDesc := NewProbeDesc("Containerd config snippets in "+importsDir, path)

			files, err := filepath.Glob(filepath.Join(importsDir, "*.toml"))
			if err != nil {
				return r.Error(
					groupDesc,
					fmt.Errorf("failed to scan containerd import directory %q: %w", importsDir, err),
				)
			}

			if len(files) == 0 {
				return r.Pass(groupDesc, StringProp("no config snippets found"))
			}

			// Emit the depth-1 header row; individual file results appear nested under it.
			if err := r.Pass(groupDesc, StringProp(importsDir)); err != nil {
				return err
			}

			for _, filePath := range files {
				// Need to clone the original path slice so we don't accindentally modify it for the next iteration of the loop.
				fileProbePath := append(slices.Clone(path), "file:"+filepath.Base(filePath))
				desc := NewProbeDesc(
					"Containerd config snippet: "+filepath.Base(filePath),
					fileProbePath,
				)

				data, err := os.ReadFile(filePath)
				if err != nil {
					if err := r.Error(desc, fmt.Errorf("failed to read containerd config snippet: %w", err)); err != nil {
						return err
					}
					continue
				}

				if err := containerd.ValidateConfigFile(data); err != nil {
					if err := r.Reject(desc, StringProp(filePath), err.Error()); err != nil {
						return err
					}
					continue
				}

				if err := r.Pass(desc, StringProp(filePath)); err != nil {
					return err
				}
			}

			return nil
		})
	})
}
