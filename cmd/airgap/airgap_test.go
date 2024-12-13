/*
Copyright 2024 k0s authors

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

package airgap_test

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/k0sproject/k0s/cmd"
	"github.com/stretchr/testify/assert"
)

func TestAirgapCmd_RejectsUnknownCommands(t *testing.T) {
	var stdout, stderr strings.Builder
	underTest := cmd.NewRootCmd()
	underTest.SetArgs([]string{"airgap", "bogus"})
	underTest.SetIn(iotest.ErrReader(errors.New("unexpected read from standard input")))
	underTest.SetOut(&stdout)
	underTest.SetErr(&stderr)

	msg := `unknown command "bogus" for "k0s airgap"`
	assert.ErrorContains(t, underTest.Execute(), msg)
	assert.Equal(t, "Error: "+msg+"\n", stderr.String())
	assert.Empty(t, stdout.String())
}
