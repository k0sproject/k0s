/*
Copyright 2020 k0s authors

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

package machineid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineIDFromHostname(t *testing.T) {
	id, err := fromHostname()
	require.NoError(t, err, "fromHostname() failed")
	assert.Len(t, id.ID(), 32)

	// test that id does not change
	id2, err := fromHostname()
	require.NoError(t, err, "fromHostname() is flaky")
	assert.Equal(t, id, id2, "fromHostname() is not deterministic")
}
