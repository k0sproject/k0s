/*
Copyright 2023 k0s authors

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

package cmd_test

import (
	"bytes"
	"io"
	"slices"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
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
