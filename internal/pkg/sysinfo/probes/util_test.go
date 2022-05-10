/*
Copyright 2022 k0s authors

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

package probes

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIECBytes_String(t *testing.T) {

	for _, data := range []struct {
		v   uint64
		str string
	}{
		{0, "0 B"},
		{1*Ki - 1, "1023 B"},
		{1 * Ki, "1.0 KiB"},
		{1*Mi - 1, "1024.0 KiB"},
		{1 * Mi, "1.0 MiB"},
		{math.MaxUint64, "16.0 EiB"},
	} {
		t.Run(strconv.FormatUint(data.v, 10), func(t *testing.T) {
			assert.Equal(t, data.str, iecBytes(data.v).String())
		})
	}
}
