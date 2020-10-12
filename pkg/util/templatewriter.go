package util

import (
	"io"
	"os"
	"github.com/Masterminds/sprig"
	"html/template"

	"github.com/Mirantis/mke/pkg/constant"
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
	podFile, err := os.OpenFile(p.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, constant.CertRootMode)
	if err != nil {
		return errors.Wrapf(err, "failed to open pod file for %s", p.Name)
	}
	return p.WriteToBuffer(podFile)
}


// WriteToBuffer writes executed template tot he given writer
func (p *TemplateWriter) WriteToBuffer(w io.Writer) error {
	t, err := template.New(p.Name).Funcs(sprig.FuncMap()).Parse(p.Template)
	if err != nil {
		return errors.Wrapf(err, "failed to parse template for %s", p.Name)
	}
	err = t.Execute(w, p.Data)
	if err != nil {
		return errors.Wrapf(err, "failed to execute template for %s", p.Name)
	}

	return nil
}
