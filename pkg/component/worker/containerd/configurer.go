// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml"
)

// marshalContainerdConfig returns the TOML bytes for the k0s-managed containerd
// configuration, with CRI plugin defaults inline and a glob import for user drop-ins.
func marshalContainerdConfig(importsPath, sandboxImage string) ([]byte, error) {
	imagesConf := map[string]any{
		"pinned_images": map[string]any{
			"sandbox": sandboxImage,
		},
	}

	runtimeConf := map[string]any{}

	if runtime.GOOS == "windows" {
		runtimeConf["cni"] = map[string]any{
			"conf_dir": `c:\etc\cni\net.d`,
			"bin_dirs": []any{`c:\opt\cni\bin`},
		}
	}

	return toml.Marshal(map[string]any{
		"version": 3,
		"imports": []string{filepath.Join(importsPath, "*.toml")},
		"plugins": map[string]any{
			"io.containerd.cri.v1.images":  imagesConf,
			"io.containerd.cri.v1.runtime": runtimeConf,
		},
	})
}

// ValidateConfigFile checks if the given config data is valid and compatible with the k0s-managed
// version of containerd. Returns a descriptive error if not.
func ValidateConfigFile(data []byte) error {
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return fmt.Errorf("failed to parse TOML: %w", err)
	}
	return validateConfigTree(tree)
}

// validateConfigTree performs compatibility checks on a parsed containerd
// configuration tree and returns a combined error if any incompatibilities are found.
func validateConfigTree(tree *toml.Tree) error {
	var errs []error

	if err := assertV3Config(tree); err != nil {
		errs = append(errs, err)
	}

	if hasV2CRIConfig(tree) {
		errs = append(errs, errors.New("configuration contains a [plugins.\"io.containerd.grpc.v1.cri\"] section which is the containerd v1 CRI plugin format"))
	}

	return errors.Join(errs...)
}

// hasV2CRIConfig checks if the given containerd configuration tree contains any plugins."io.containerd.grpc.v1.cri"
// configuration section. This is the main difference between containerd v2 and v3 configuration formats, so if this is present we
// want the user to fix it manually instead of trying to merge it and potentially causing unexpected breakage.
func hasV2CRIConfig(tree *toml.Tree) bool {
	return tree.HasPath([]string{"plugins", "io.containerd.grpc.v1.cri"})
}

func assertV3Config(tree *toml.Tree) error {
	version := tree.Get("version")
	if version != nil {
		v, ok := version.(int64)
		if !ok {
			return fmt.Errorf("unexpected type for version field: expected int64, got %T", version)
		}

		if v != 3 {
			return fmt.Errorf("unsupported configuration version: expected 3, got %d", v)
		}
	}
	return nil
}
