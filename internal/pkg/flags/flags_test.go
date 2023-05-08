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

package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagSplitting(t *testing.T) {
	args := "--foo=bar --foobar=xyz,asd --bool-flag"

	m := Split(args)

	assert.Equal(t, 3, len(m))
	assert.Equal(t, "bar", m["--foo"])
	assert.Equal(t, "xyz,asd", m["--foobar"])
	assert.Equal(t, "", m["--bool-flag"])
}

func TestFlagSplittingBoolFlags(t *testing.T) {
	args := "--bool-flag"

	m := Split(args)

	assert.Equal(t, 1, len(m))
	assert.Equal(t, "", m["--bool-flag"])
}
