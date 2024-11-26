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

package sysinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/term"
	"sigs.k8s.io/yaml"
)

func NewSysinfoCmd() *cobra.Command {

	var sysinfoSpec sysinfo.K0sSysinfoSpec
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "sysinfo",
		Short: "Display system information",
		Long:  `Runs k0s's pre-flight checks and issues the results to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sysinfoSpec.AddDebugProbes = true
			probes := sysinfoSpec.NewSysinfoProbes()
			out := cmd.OutOrStdout()

			var cli cliReporter
			switch outputFormat {
			case "text":
				cli = &humanReporter{
					colors: aurora.NewAurora(term.IsTerminal(out)),
				}
			case "json":
				cli = &jsonReporter{}
			case "yaml":
				cli = &yamlReporter{}
			default:
				return fmt.Errorf("unknown output format: %q", outputFormat)
			}

			if err := probes.Probe(cli); err != nil {
				return err
			}
			if err := cli.printResults(out); err != nil {
				return err
			}

			if cli.isFailed() {
				return errors.New("sysinfo failed")
			}
			return nil
		},
	}

	// append flags
	flags := cmd.Flags()
	flags.BoolVar(&sysinfoSpec.ControllerRoleEnabled, "controller", true, "Include controller-specific sysinfo")
	flags.BoolVar(&sysinfoSpec.WorkerRoleEnabled, "worker", true, "Include worker-specific sysinfo")
	flags.StringVar(&sysinfoSpec.DataDir, "data-dir", constant.DataDirDefault, "Data Directory for k0s")
	flags.StringVarP(&outputFormat, "output", "o", "text", "Output format (valid values: text, json, yaml)")

	return cmd
}

type cliReporter interface {
	probes.Reporter
	isFailed() bool
	printResults(io.Writer) error
}

type humanReporter struct {
	resultsCollector
	colors aurora.Aurora
}

func (r *humanReporter) printResults(w io.Writer) error {
	for _, p := range r.results {
		if err := r.printOneHuman(w, p); err != nil {
			return err
		}
	}
	return nil
}

func (r *humanReporter) printOneHuman(w io.Writer, p Probe) error {
	var out string
	switch p.Category {
	case ProbeCategoryPass:
		out = aurora.Sprintf("%s%s%s%s\n",
			indent(p.Path),
			r.colors.BrightWhite(p.DisplayName+": "),
			r.colors.Green(p.Prop),
			buildMsg(p.Prop, string(p.Category), p.Message))
	case ProbeCategoryWarning:
		out = aurora.Sprintf("%s%s%s%s\n",
			indent(p.Path),
			r.colors.BrightWhite(p.DisplayName+": "),
			r.colors.Yellow(p.Prop),
			buildMsg(p.Prop, string(p.Category), p.Message))
	case ProbeCategoryRejected:
		out = aurora.Sprintf("%s%s%s%s\n",
			indent(p.Path),
			r.colors.BrightWhite(p.DisplayName+": "),
			r.colors.Bold(r.colors.Red(p.Prop)),
			buildMsg(p.Prop, string(p.Category), p.Message))
	case ProbeCategoryError:
		errStr := "error"
		if p.Error != nil {
			e := p.Error.Error()
			if e != "" {
				errStr = errStr + ": " + e
			}
		}

		out = aurora.Sprintf("%s%s%s\n",
			indent(p.Path),
			r.colors.BrightWhite(p.DisplayName+": "),
			r.colors.Bold(errStr).Red(),
		)
	default:
		return fmt.Errorf("unknown probe category %q", p.Category)
	}
	_, err := io.WriteString(w, out)
	return err
}

type jsonReporter struct {
	resultsCollector
}

func (r *jsonReporter) printResults(w io.Writer) error {
	jsn, err := json.MarshalIndent(r.results, "", "   ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(jsn))
	return err
}

type yamlReporter struct {
	resultsCollector
}

func (r *yamlReporter) printResults(w io.Writer) error {
	ym, err := yaml.Marshal(r.results)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(ym))
	return err
}

type Probe struct {
	Path        []string
	DisplayName string
	Prop        string
	Message     string
	Category    ProbeCategory
	Error       error
}

type ProbeCategory string

const (
	ProbeCategoryPass     ProbeCategory = "pass"
	ProbeCategoryWarning  ProbeCategory = "warning"
	ProbeCategoryRejected ProbeCategory = "rejected"
	ProbeCategoryError    ProbeCategory = "error"
)

type resultsCollector struct {
	results []Probe
	failed  bool
}

func (r *resultsCollector) Pass(p probes.ProbeDesc, v probes.ProbedProp) error {
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Prop:        propString(v),
		Category:    ProbeCategoryPass,
	})
	return nil
}

func (r *resultsCollector) Warn(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Prop:        propString(v),
		Message:     msg,
		Category:    ProbeCategoryWarning,
	})
	return nil
}

func (r *resultsCollector) Reject(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	r.failed = true
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Prop:        propString(v),
		Message:     msg,
		Category:    ProbeCategoryRejected,
	})
	return nil
}

func (r *resultsCollector) Error(p probes.ProbeDesc, err error) error {
	r.failed = true
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Category:    ProbeCategoryError,
		Error:       err,
	})
	return nil
}

func (r *resultsCollector) isFailed() bool {
	return r.failed
}

func probePath(p probes.ProbeDesc) []string {
	if len(p.Path()) == 0 {
		return nil
	}
	return p.Path()
}

func propString(p probes.ProbedProp) string {
	if p == nil {
		return ""
	}

	return p.String()
}

func indent(path []string) string {
	count := len(path) - 1
	if count < 1 {
		return ""
	}

	return strings.Repeat("  ", count)
}

func buildMsg(propString, category, msg string) string {
	var buf strings.Builder
	if propString != "" {
		buf.WriteRune(' ')
	}
	buf.WriteRune('(')
	buf.WriteString(category)
	if msg != "" {
		buf.WriteString(": ")
		buf.WriteString(msg)
	}
	buf.WriteRune(')')
	return buf.String()
}
