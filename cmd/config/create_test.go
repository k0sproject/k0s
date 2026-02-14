// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
