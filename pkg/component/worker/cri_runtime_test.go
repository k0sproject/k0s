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

package worker_test

import (
	"runtime"
	"testing"

	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/stretchr/testify/assert"
)

func TestGetContainerRuntimeEndpoint_Defaults(t *testing.T) {
	runtimeType, runtimeEndpoint, err := worker.GetContainerRuntimeEndpoint("", "/run/user/999")
	assert.NoError(t, err)
	assert.Empty(t, runtimeType)
	if assert.NotNil(t, runtimeEndpoint) {
		if runtime.GOOS == "windows" {
			assert.Equal(t, "npipe:////./pipe/containerd-containerd", runtimeEndpoint.String())
			assert.Equal(t, "//./pipe/containerd-containerd", runtimeEndpoint.Path)
		} else {
			assert.Equal(t, "unix:///run/user/999/containerd.sock", runtimeEndpoint.String())
			assert.Equal(t, "/run/user/999/containerd.sock", runtimeEndpoint.Path)
		}
	}
}

func TestGetContainerRuntimeEndpoint_Flag(t *testing.T) {
	cases := []struct {
		name        string
		flag        string
		expType     string
		expEndpoint string
		expPath     string
		err         string
	}{
		{
			name:        "docker",
			flag:        "docker:unix:///var/run/docker.sock",
			expType:     "docker",
			expEndpoint: "unix:///var/run/docker.sock",
			expPath:     "/var/run/docker.sock",
			err:         "",
		},
		{
			name:        "containerd-unix",
			flag:        "remote:unix:///var/run/mke/containerd.sock",
			expType:     "remote",
			expEndpoint: "unix:///var/run/mke/containerd.sock",
			expPath:     "/var/run/mke/containerd.sock",
			err:         "",
		},
		{
			name:        "containerd-windows",
			flag:        "remote:npipe:////./pipe/containerd-containerd",
			expEndpoint: "npipe:////./pipe/containerd-containerd",
			expPath:     "//./pipe/containerd-containerd",
			err:         "",
		},
		{
			name:        "no-colon-in-flag",
			flag:        "no-colon-in-flag",
			expType:     "",
			expEndpoint: "",
			expPath:     "",
			err:         "cannot parse CRI socket flag",
		},
		{
			name:        "invalid-url",
			flag:        "remote:u<nix:///foo",
			expType:     "",
			expEndpoint: "",
			expPath:     "",
			err:         "failed to parse runtime endpoint: ",
		},
		{
			name:        "unknown-type",
			flag:        "foobar:unix:///var/run/mke/containerd.sock",
			expType:     "",
			expEndpoint: "",
			expPath:     "",
			err:         "unknown runtime type foobar, must be either of remote or docker",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			criType, endpoint, err := worker.GetContainerRuntimeEndpoint(tc.flag, "y u use me?")
			if tc.err != "" {
				assert.ErrorContains(t, err, tc.err)
				assert.Empty(t, criType)
				assert.Nil(t, endpoint)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expType, criType)
				if assert.NotNil(t, endpoint) {
					assert.Equal(t, tc.expEndpoint, endpoint.String())
					assert.Equal(t, tc.expPath, endpoint.Path)
				}
			}
		})
	}

}
