// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package templatewriter

import (
	"fmt"
	"io"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
)

// TemplateWriter is a helper to write templated kube manifests
type TemplateWriter struct {
	Name     string
	Template string
	Data     any
	Path     string
}

// Write executes the template and writes the results on disk
func (p *TemplateWriter) Write() error {
	return file.WriteAtomically(p.Path, constant.CertMode, p.WriteToBuffer)
}

// WriteToBuffer writes executed template tot he given writer
func (p *TemplateWriter) WriteToBuffer(w io.Writer) error {
	t, err := template.New(p.Name).Funcs(sprig.TxtFuncMap()).Parse(p.Template)
	if err != nil {
		return fmt.Errorf("failed to parse template for %s: %w", p.Name, err)
	}
	err = t.Execute(w, p.Data)
	if err != nil {
		return fmt.Errorf("failed to execute template for %s: %w", p.Name, err)
	}

	return nil
}
