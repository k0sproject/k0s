// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/k0sproject/k0s/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCmd_Flags ensures that no unwanted global flags have been registered
// and leak into k0s. This happens rather quickly, e.g. if some dependency puts
// stuff into pflag.CommandLine.
func TestRootCmd_Flags(t *testing.T) {
	expectedVisibleFlags := []string{"help"}
	expectedHiddenFlags := []string{
		"version", // registered by k0scloudprovider; unwanted but unavoidable
	}

	var stderr bytes.Buffer

	underTest := cmd.NewRootCmd()
	underTest.SetArgs(nil)
	underTest.SetOut(io.Discard) // Don't care about the usage output here
	underTest.SetErr(&stderr)

	err := underTest.Execute()

	assert.NoError(t, err)
	assert.Empty(t, stderr.String(), "Something has been written to stderr")

	// This has to happen after the command has been executed.
	// Cobra will have populated everything by then.
	var visibleFlags []string
	var hiddenFlags []string
	underTest.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			hiddenFlags = append(hiddenFlags, f.Name)
		} else {
			visibleFlags = append(visibleFlags, f.Name)
		}
	})

	slices.Sort(visibleFlags)
	slices.Sort(hiddenFlags)

	assert.Equal(t, expectedVisibleFlags, visibleFlags, "visible flags changed unexpectedly")
	assert.Equal(t, expectedHiddenFlags, hiddenFlags, "hidden flags changed unexpectedly")
}

func TestUnknownSubCommandsAreRejected(t *testing.T) {
	commandsWithArguments := []string{
		"airgap bundle-artifacts",
		"kubeconfig create",
		"token invalidate",
		"worker",
	}
	if runtime.GOOS == "linux" {
		commandsWithArguments = append(commandsWithArguments,
			"controller",
			"restore",
		)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			assert.Empty(t, commandsWithArguments, "Some sub-commands are listed unnecessarily")
		}
	})

	shouldBeTested := func(t *testing.T, underTest *cobra.Command, subCommand string) {
		if underTest.ValidArgs != nil {
			t.Skipf("has ValidArgs: %v", underTest.ValidArgs)
		}
		if underTest.Deprecated != "" {
			t.Skipf("is deprecated")
		}

		if idx := slices.Index(commandsWithArguments, subCommand); idx >= 0 {
			commandsWithArguments = slices.Delete(commandsWithArguments, idx, idx+1)
			t.Skip("accepts arguments")
		}
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("If this sub-command accepts arguments, include %q in the above list", subCommand)
			}
		})
	}

	var testCommand func(underTest *cobra.Command, args []string) func(t *testing.T)
	testCommand = func(underTest *cobra.Command, args []string) func(t *testing.T) {
		return func(t *testing.T) {
			for _, cmd := range underTest.Commands() {
				name, _, _ := strings.Cut(cmd.Use, " ")
				require.NotEmpty(t, name)
				switch name {
				case "member-update": // Don't test comands with positional args
				default:
					t.Run(name, testCommand(cmd, slices.Concat(args, []string{name})))
				}
			}

			subCommand := strings.Join(args, " ")
			shouldBeTested(t, underTest, subCommand)

			var stdout, stderr strings.Builder
			// Reset any "required" annotations on flags
			underTest.Flags().VisitAll(func(flag *pflag.Flag) {
				flag.Annotations = nil
			})

			root := cmd.NewRootCmd()

			root.SetArgs(slices.Concat(args, []string{"bogus"}))
			root.SetIn(iotest.ErrReader(errors.New("unexpected read from standard input")))
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			msg := fmt.Sprintf(`unknown command "bogus" for "k0s %s"`, subCommand)
			assert.ErrorContains(t, root.Execute(), msg)
			assert.Equal(t, "Error: "+msg+"\n", stderr.String())
			assert.Empty(t, stdout.String())
		}
	}

	underTest := cmd.NewRootCmd()
	for _, cmd := range underTest.Commands() {
		name, _, _ := strings.Cut(cmd.Use, " ")
		require.NotEmpty(t, name)

		switch name {
		case "ctr", "kubectl": // Don't test embedded sub-commands
		default:
			t.Run(name, testCommand(cmd, []string{name}))
		}
	}
}
