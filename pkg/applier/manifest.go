// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// WriteManifest writes the objects as a multi-document YAML file, unless it's up to date.
// Unchanged manifests aren't written again, so that the applier doesn't see a change.
func WriteManifest(path string, objects []runtime.Object) error {
	var buf bytes.Buffer
	for i, obj := range objects {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(data)
	}
	if existing, err := os.ReadFile(filepath.Clean(path)); err == nil && bytes.Equal(existing, buf.Bytes()) {
		return nil
	}
	return file.AtomicWithTarget(path).Write(buf.Bytes())
}
