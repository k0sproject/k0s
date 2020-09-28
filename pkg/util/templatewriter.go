package util

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
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
	err := InitDirectory(filepath.Dir(p.Path), 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p.Path))
	}

	t, err := template.New(p.Name).Parse(p.Template)
	if err != nil {
		return errors.Wrapf(err, "failed to parse template for %s", p.Name)
	}

	podFile, err := os.OpenFile(p.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to open pod file for %s", p.Name)
	}

	err = t.Execute(podFile, p.Data)
	if err != nil {
		return errors.Wrapf(err, "failed to execute template for %s", p.Name)
	}

	return nil
}
