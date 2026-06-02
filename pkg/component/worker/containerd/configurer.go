// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mesosphere/toml-merge/pkg/patch"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
)

// Resolved and merged containerd configuration data.
type resolvedConfig struct {
	// Serialized configuration including merged CRI plugin configuration data.
	CRIConfig string

	// Paths to additional partial configuration files to be imported. Those
	// files won't contain any CRI plugin configuration data.
	ImportPaths []string
}

type configurer struct {
	loadPath   string
	pauseImage string

	log *logrus.Entry
}

// Resolves partial containerd configuration files from the import glob path. If
// a file contains a CRI plugin configuration section, it will be merged into
// k0s's default configuration, if not, it will be added to the list of import
// paths.
func (c *configurer) handleImports() (*resolvedConfig, error) {
	var importPaths []string

	defaultConfig, err := generateDefaultCRIConfig(c.pauseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate containerd default CRI config: %w", err)
	}

	filePaths, err := filepath.Glob(c.loadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to look for containerd import files: %w", err)
	}
	c.log.Debugf("found containerd config files to import: %v", filePaths)

	// Since the default config contains configuration data for the CRI plugin,
	// and containerd has decided to replace rather than merge individual plugin
	// configuration sections from imported config files, we need to manually
	// take care of merging k0s's defaults with the user overrides. Loop through
	// all import files and check if they contain any CRI plugin config. If they
	// do, treat them as merge patches to the default config, if they don't,
	// just add them as normal imports to be handled by containerd.
	finalConfig := string(defaultConfig)
	errs := []error{}
	for _, filePath := range filePaths {
		c.log.Debugf("Processing containerd configuration file %s", filePath)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		tree, err := toml.LoadBytes(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse TOML in %q: %w", filePath, err))
			continue
		}

		isV3Config := true
		if err := validateConfigTree(tree); err != nil {
			errs = append(errs, err)
			isV3Config = false
		}

		if isV3Config && hasCRIPluginConfig(tree) {
			c.log.Infof("Found CRI plugin configuration in %q, treating as merge patch", filePath)
			finalConfig, err = patch.TOMLString(finalConfig, patch.FilePatches(filePath))
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to merge data from %s into containerd configuration: %w", filePath, err))
			}
		} else {
			c.log.Debugf("No CRI plugin configuration found in %s, adding as-is to imports", filePath)
			importPaths = append(importPaths, filePath)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("encountered errors while processing containerd configuration import files: %v", errs)
	}

	return &resolvedConfig{CRIConfig: finalConfig, ImportPaths: importPaths}, nil
}

// Returns the default containerd config, including only the CRI plugin
// configuration, using the given image for sandbox containers. Uses the
// containerd package to generate all the rest, so this will be in sync with
// containerd's defaults for the CRI plugin.
func generateDefaultCRIConfig(sandboxContainerImage string) ([]byte, error) {
	// https://github.com/containerd/containerd/blob/main/docs/cri/config.md#full-configuration
	imagesConf := map[string]any{
		"pinned_images": map[string]any{
			"sandbox": sandboxContainerImage,
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
		"plugins": map[string]any{
			"io.containerd.cri.v1.images":  imagesConf,
			"io.containerd.cri.v1.runtime": runtimeConf,
		},
	})
}

func hasCRIPluginConfig(tree *toml.Tree) bool {
	return tree.HasPath([]string{"plugins", "io.containerd.cri.v1.runtime"}) || tree.HasPath([]string{"plugins", "io.containerd.cri.v1.images"})
}

// Checks if the given config data is valid and compatible with the k0s-managed
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
