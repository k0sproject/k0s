/*
Copyright 2020 Mirantis, Inc.

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
package sonobuoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testOutput = `
Plugin: e2e
Status: passed
Total: 5232
Passed: 32
Failed: 0
Skipped: 5200
`

func TestParsing(t *testing.T) {
	a := assert.New(t)

	r, err := ResultFromString(testOutput)

	a.NoError(err)

	a.Equal("e2e", r.Plugin)
	a.Equal("passed", r.Status)
	a.Equal(5232, r.Total)
	a.Equal(0, r.Failed)
	a.Equal(5200, r.Skipped)

}
