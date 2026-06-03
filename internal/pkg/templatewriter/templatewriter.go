// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package templatewriter

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/patches"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// TemplateWriter is a helper to write templated kube manifests
type TemplateWriter struct {
	Name     string
	Template string
	Data     any
	Path     string
	// Patches are applied to the rendered manifest before writing. nil = no-op.
	Patches v1beta1.Patches
}

// Write executes the template and writes the results on disk
func (p *TemplateWriter) Write() error {
	return file.WriteAtomically(p.Path, constant.CertMode, p.WriteToBuffer)
}

// WriteToBuffer writes the executed template (with patches applied) to w.
func (p *TemplateWriter) WriteToBuffer(w io.Writer) error {
	t, err := template.New(p.Name).Funcs(sprig.TxtFuncMap()).Parse(p.Template)
	if err != nil {
		return fmt.Errorf("failed to parse template for %s: %w", p.Name, err)
	}

	var rendered bytes.Buffer
	if err := t.Execute(&rendered, p.Data); err != nil {
		return fmt.Errorf("failed to execute template for %s: %w", p.Name, err)
	}

	out := rendered.Bytes()
	if len(p.Patches) > 0 {
		out, err = patches.Apply(out, p.Patches)
		if err != nil {
			return fmt.Errorf("failed to apply patches for %s: %w", p.Name, err)
		}
	}

	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("failed to write manifest for %s: %w", p.Name, err)
	}

	return nil
}
