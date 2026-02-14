// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"

	internallog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type DebugFlags struct {
	logsToStdout     bool
	verbose          bool
	verboseByDefault bool
	debug            bool
	debugListenOn    string
}

func (f *DebugFlags) IsDebug() bool {
	return f.debug
}

// Configures the debug flags for long-running commands.
// Must be called before adding the flags to a FlagSet.
func (f *DebugFlags) LongRunning() *DebugFlags {
	f.logsToStdout = true

	// The default value won't be reflected in the flag set
	// once the flags have been added.
	f.verboseByDefault = true

	return f
}

// Adds the debug flags to the given FlagSet.
func (f *DebugFlags) AddToFlagSet(flags *pflag.FlagSet) {
	flags.BoolVarP(&f.verbose, "verbose", "v", f.verboseByDefault, "Verbose logging")
	flags.BoolVarP(&f.debug, "debug", "d", false, "Debug logging (implies verbose logging)")
	flags.StringVar(&f.debugListenOn, "debugListenOn", ":6060", "Http listenOn for Debug pprof handler")
}

// Adds the debug flags to the given FlagSet when in "kubectl" mode.
// This won't use shorthands, as this will interfere with kubectl's flags.
func (f *DebugFlags) AddToKubectlFlagSet(flags *pflag.FlagSet) {
	debugDefault := false
	if v, ok := os.LookupEnv("DEBUG"); ok {
		debugDefault, _ = strconv.ParseBool(v)
	}

	flags.BoolVar(&f.debug, "debug", debugDefault, "Debug logging [$DEBUG]")
}

func (f *DebugFlags) Run(cmd *cobra.Command, _ []string) {
	if f.logsToStdout {
		// Only switch the logrus backend if there's no specific backend
		// installed already. Setting the logrus output would effectively remove
		// that backend again.
		if !k0scontext.HasValue[internallog.Backend](cmd.Context()) {
			logrus.SetOutput(cmd.OutOrStdout())
		}
	}

	switch {
	case f.debug:
		internallog.SetDebugLevel()

		if f.verbose {
			if !f.verboseByDefault {
				logrus.Debug("--debug already implies --verbose")
			}
		} else if f.verboseByDefault {
			logrus.Debug("--debug overrides --verbose=false")
		}

		go func() {
			log := logrus.WithField("debug_server", f.debugListenOn)
			log.Debug("Starting debug server")
			if err := http.ListenAndServe(f.debugListenOn, nil); !errors.Is(err, http.ErrServerClosed) {
				log.WithError(err).Debug("Failed to start debug server")
			} else {
				log.Debug("Debug server closed")
			}
		}()

	case f.verbose:
		internallog.SetInfoLevel()
	}
}
