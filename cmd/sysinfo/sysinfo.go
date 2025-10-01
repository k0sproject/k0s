// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/term"
	"sigs.k8s.io/yaml"
)

func NewSysinfoCmd() *cobra.Command {
	var (
		debugFlags   internal.DebugFlags
		sysinfoSpec  sysinfo.K0sSysinfoSpec
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:              "sysinfo",
		Short:            "Display system information",
		Long:             `Runs k0s's pre-flight checks and issues the results to stdout.`,
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sysinfoSpec.AddDebugProbes = true
			probes := sysinfoSpec.NewSysinfoProbes()
			out := cmd.OutOrStdout()

			switch outputFormat {
			case "text":
				cli := &cliReporter{
					w:      out,
					colors: aurora.NewAurora(term.IsTerminal(out)),
				}
				if err := probes.Probe(cli); err != nil {
					return err
				}
				if cli.failed {
					return errors.New("sysinfo failed")
				}
				return nil

			case "json":
				return collectAndPrint(probes, out, func(v any) ([]byte, error) {
					return json.MarshalIndent(v, "", "  ")
				})

			case "yaml":
				return collectAndPrint(probes, out, yaml.Marshal)

			default:
				return fmt.Errorf("unknown output format: %q", outputFormat)
			}
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	// append flags
	flags := cmd.Flags()
	flags.BoolVar(&sysinfoSpec.ControllerRoleEnabled, "controller", true, "Include controller-specific sysinfo")
	flags.BoolVar(&sysinfoSpec.WorkerRoleEnabled, "worker", true, "Include worker-specific sysinfo")
	flags.StringVar(&sysinfoSpec.DataDir, "data-dir", constant.DataDirDefault, "Data Directory for k0s")
	flags.StringVarP(&outputFormat, "output", "o", "text", "Output format (valid values: text, json, yaml)")

	return cmd
}

type cliReporter struct {
	w      io.Writer
	colors aurora.Aurora
	failed bool
}

func (r *cliReporter) Pass(p probes.ProbeDesc, v probes.ProbedProp) error {
	prop := propString(v)
	return r.printf("%s%s%s%s\n",
		indent(p),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Green(prop),
		buildMsg(prop, "pass", ""),
	)
}

func (r *cliReporter) Warn(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	prop := propString(v)
	return r.printf("%s%s%s%s\n",
		indent(p),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Yellow(prop),
		buildMsg(prop, "warning", msg))
}

func (r *cliReporter) Reject(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	r.failed = true
	prop := propString(v)
	return r.printf("%s%s%s%s\n",
		indent(p),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Bold(r.colors.Red(prop)),
		buildMsg(prop, "rejected", msg))
}

func (r *cliReporter) Error(p probes.ProbeDesc, err error) error {
	r.failed = true

	errStr := "error"
	if err != nil {
		e := err.Error()
		if e != "" {
			errStr = errStr + ": " + e
		}
	}

	return r.printf("%s%s%s\n",
		indent(p),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Bold(errStr).Red(),
	)
}

func collectAndPrint(probe probes.Probe, out io.Writer, marshal func(any) ([]byte, error)) error {
	var c resultsCollector
	if err := probe.Probe(&c); err != nil {
		return err
	}
	bytes, err := marshal(c.results)
	if err != nil {
		return err
	}
	_, err = out.Write(bytes)
	if err != nil {
		return err
	}
	if c.failed {
		return errors.New("sysinfo failed")
	}
	return nil
}

func (r *cliReporter) printf(format any, args ...any) error {
	_, err := io.WriteString(r.w, aurora.Sprintf(format, args...))
	return err
}

type Probe struct {
	Path        []string      `json:"path"`
	DisplayName string        `json:"displayName"`
	Prop        string        `json:"prop"`
	Message     string        `json:"message"`
	Category    ProbeCategory `json:"category"`
	Error       error         `json:"error"`
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

func indent(p probes.ProbeDesc) string {
	count := 0
	if p != nil {
		count = len(p.Path()) - 1
		if count < 1 {
			return ""
		}
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
