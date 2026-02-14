// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"

	"github.com/stretchr/testify/assert"
)

func TestFlagSplitting(t *testing.T) {
	args := "--foo=bar --foobar=xyz,asd --bool-flag"

	m := Split(args)

	assert.Equal(t, stringmap.StringMap{
		"--foo":       "bar",
		"--foobar":    "xyz,asd",
		"--bool-flag": "",
	}, m)
}

func TestFlagSplittingBoolFlags(t *testing.T) {
	args := "--bool-flag"

	m := Split(args)

	assert.Equal(t, stringmap.StringMap{"--bool-flag": ""}, m)
}
