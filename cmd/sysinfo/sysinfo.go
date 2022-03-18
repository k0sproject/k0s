/*
Copyright 2022 k0s authors

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
	"errors"
	"io"
	"os"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/term"
)

func NewSysinfoCmd() *cobra.Command {

	var sysinfoSpec sysinfo.K0sSysinfoSpec

	cmd := &cobra.Command{
		Use:   "sysinfo",
		Short: "Display system information",
		Long:  `Runs k0s's pre-flight checks and issues the results to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			sysinfoSpec.AddDebugProbes = true
			probes := sysinfoSpec.NewSysinfoProbes()
			cli := &cliReporter{
				w:      os.Stdout,
				colors: aurora.NewAurora(term.IsTerminal(os.Stdout)),
			}

			if err := probes.Probe(cli); err != nil {
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

	return cmd
}

type cliReporter struct {
	w      io.Writer
	colors aurora.Aurora
	failed bool
}

func (r *cliReporter) Pass(p probes.ProbeDesc, v probes.ProbedProp) error {
	return r.printf("%s%s%s (pass)\n",
		strings.Repeat("  ", len(p.Path())-1),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Green(v.String()))
}

func (r *cliReporter) Warn(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	if msg == "" {
		msg = " (warning)"
	} else {
		msg = " (warning: " + msg + ")"
	}

	return r.printf("%s%s%s%s\n",
		strings.Repeat("  ", len(p.Path())-1),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Yellow(v.String()),
		msg)
}

func (r *cliReporter) Reject(p probes.ProbeDesc, v probes.ProbedProp, msg string) error {
	r.failed = true
	if msg == "" {
		msg = " (rejected)"
	} else {
		msg = " (rejected: " + msg + ")"
	}

	return r.printf("%s%s%s%s\n",
		strings.Repeat("  ", len(p.Path())-1),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Bold(r.colors.Red(v.String())),
		msg)
}

func (r *cliReporter) Error(p probes.ProbeDesc, err error) error {
	r.failed = true
	return r.printf("%s%s%s\n",
		strings.Repeat("  ", len(p.Path())-1),
		r.colors.BrightWhite(p.DisplayName()+": "),
		r.colors.Bold(r.colors.Red("error: "+err.Error())))
}

func (r *cliReporter) printf(format interface{}, args ...interface{}) error {
	_, err := io.WriteString(r.w, aurora.Sprintf(format, args...))
	return err
}
