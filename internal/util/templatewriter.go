/*
Copyright 2021 k0s authors

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
package util

import (
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/k0sproject/k0s/pkg/constant"
)

// TemplateWriter is a helper to write templated kube manifests
type TemplateWriter struct {
	Name     string
	Template string
	Data     interface{}
	Path     string
}

// Write writes executes the template and writes the results on disk
func (p *TemplateWriter) Write() error {
	podFile, err := os.OpenFile(p.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, constant.CertMode)
	if err != nil {
		return fmt.Errorf("failed to open pod file for %s: %w", p.Name, err)
	}
	defer podFile.Close()
	return p.WriteToBuffer(podFile)
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
