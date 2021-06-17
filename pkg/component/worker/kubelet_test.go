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
package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCRISocketParsing(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		expType   string
		expSocket string
		err       bool
	}{
		{
			name:      "docker",
			input:     "docker:unix:///var/run/docker.sock",
			expType:   "docker",
			expSocket: "unix:///var/run/docker.sock",
			err:       false,
		},
		{
			name:      "containerd",
			input:     "remote:unix:///var/run/mke/containerd.sock",
			expType:   "remote",
			expSocket: "unix:///var/run/mke/containerd.sock",
			err:       false,
		},
		{
			name:      "unknown-type",
			input:     "foobar:unix:///var/run/mke/containerd.sock",
			expType:   "remote",
			expSocket: "unix:///var/run/mke/containerd.sock",
			err:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			criType, sock, err := SplitRuntimeConfig(tc.input)
			if tc.err {
				require.Error(t, err)
			} else {
				require.Equal(t, tc.expType, criType)
				require.Equal(t, tc.expSocket, sock)
			}
		})
	}

}
