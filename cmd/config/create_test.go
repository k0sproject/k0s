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

package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k0sproject/k0s/cmd/config"
)

func TestCreateCmd_Images(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		check func(t *testing.T, cfg, needle string)
	}{
		{
			"default", []string{},
			func(t *testing.T, cfg, needle string) { t.Helper(); assert.NotContains(t, cfg, needle) },
		},

		{
			"include_images", []string{"--include-images"},
			func(t *testing.T, cfg, needle string) { t.Helper(); assert.Contains(t, cfg, needle) },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			underTest := config.NewCreateCmd()

			var out strings.Builder
			var err strings.Builder
			underTest.SetArgs(test.args)
			underTest.SetOut(&out)
			underTest.SetErr(&err)

			assert.NoError(t, underTest.Execute())

			assert.Empty(t, err.String(), "Something has been written to stderr")
			// This is a very broad check if there's some ImageSpec in the output. May
			// produce false positives if something similar gets added to the config in
			// the future. Can be refined then.
			cfg := out.String()
			test.check(t, cfg, "quay.io/k0sproject")
			test.check(t, cfg, "image: ")
			test.check(t, cfg, "version: ")
		})
	}
}
