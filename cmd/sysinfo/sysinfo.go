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
			cli := &cliReporter{
				colors: aurora.NewAurora(term.IsTerminal(out)),
			}
			if err := probes.Probe(cli); err != nil {
				return err
			}
			if err := cli.printResults(out, outputFormat); err != nil {
				return err
			}

			if cli.failed {
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
	flags.StringVarP(&outputFormat, "output", "o", "human", "Output format. Must be one of human|yaml|json")

	return cmd
}

type cliReporter struct {
	results []Probe
	colors  aurora.Aurora
	failed  bool
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

func (r *cliReporter) Pass(p probes.ProbeDesc, v probes.ProbedProp) error {
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Prop:        propString(v),
		Category:    ProbeCategoryPass,
	})
	return nil
}

func (r *cliReporter) Warn(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Prop:        propString(v),
		Message:     msg,
		Category:    ProbeCategoryWarning,
	})
	return nil
}

func (r *cliReporter) Reject(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
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

func (r *cliReporter) Error(p probes.ProbeDesc, err error) error {
	r.failed = true
	r.results = append(r.results, Probe{
		Path:        probePath(p),
		DisplayName: p.DisplayName(),
		Category:    ProbeCategoryError,
		Error:       err,
	})
	return nil
}

func (r *cliReporter) printResults(w io.Writer, outputFormat string) error {
	switch outputFormat {
	case "human":
		return r.printHuman(w)
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r.results)
	case "yaml":
		b, err := yaml.Marshal(r.results)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, string(b))
		return err
	default:
		return fmt.Errorf("unknown output format: %q", outputFormat)
	}
}

func (r *cliReporter) printHuman(w io.Writer) error {
	for _, p := range r.results {
		if err := r.printOneHuman(w, p); err != nil {
			return err
		}
	}
	return nil
}

func (r *cliReporter) printOneHuman(w io.Writer, p Probe) error {
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
