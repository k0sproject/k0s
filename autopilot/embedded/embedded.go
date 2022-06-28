// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embedded

import (
	"bytes"
	"embed"
	"fmt"
	"path"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	embeddedManifestPath = "manifests/autopilot.k0sproject.io/v1beta2"
)

var (
	//go:embed manifests/autopilot.k0sproject.io/v1beta2/*.yaml
	crds embed.FS
)

// LoadCustomResourceDefinitions extracts all of the embedded CRDs into a map, keyed
// by the real CRD name. Failing to load or parse a CRD will result in an error.
func LoadCustomResourceDefinitions() (map[string]string, error) {
	manifests := make(map[string]string)

	entries, err := crds.ReadDir(embeddedManifestPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get the CRD directory contents: %w", err)
	}

	for _, entry := range entries {
		manifestFile := path.Join(embeddedManifestPath, entry.Name())
		manifest, err := crds.ReadFile(manifestFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read '%s': %w", manifestFile, err)
		}

		// Do a parse of the CRD in order to get the proper name, paying attention
		// to (and ignoring) any empty documents.
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1024)
		var crd extensionsv1.CustomResourceDefinition

		for crd.Kind != "CustomResourceDefinition" {
			if err := decoder.Decode(&crd); err != nil {
				return nil, fmt.Errorf("unable to parse '%s': %w", manifestFile, err)
			}
		}

		manifests[crd.Name] = string(manifest)
	}

	return manifests, nil
}
