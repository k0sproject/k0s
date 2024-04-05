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
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

type RuntimeType = string
type RuntimeEndpoint = url.URL

// Parses the CRI runtime flag and returns the parsed values.
// If the flag is empty, provide k0s's defaults.
func GetContainerRuntimeEndpoint(criSocketFlag, k0sRunDir string) (RuntimeType, *RuntimeEndpoint, error) {
	switch {
	case criSocketFlag != "":
		return parseCRISocketFlag(criSocketFlag)
	case runtime.GOOS == "windows":
		return "", &url.URL{Scheme: "npipe", Path: "//./pipe/containerd-containerd"}, nil
	default:
		socketPath := filepath.Join(k0sRunDir, "containerd.sock")
		return "", &url.URL{Scheme: "unix", Path: filepath.ToSlash(socketPath)}, nil
	}
}

func parseCRISocketFlag(criSocketFlag string) (RuntimeType, *RuntimeEndpoint, error) {
	runtimeConfig := strings.SplitN(criSocketFlag, ":", 2)
	if len(runtimeConfig) != 2 {
		return "", nil, fmt.Errorf("cannot parse CRI socket flag")
	}
	runtimeType := runtimeConfig[0]
	runtimeEndpoint := runtimeConfig[1]
	if runtimeType != "docker" && runtimeType != "remote" {
		return "", nil, fmt.Errorf("unknown runtime type %s, must be either of remote or docker", runtimeType)
	}

	parsedRuntimeEndpoint, err := url.Parse(runtimeEndpoint)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse runtime endpoint: %w", err)
	}

	return runtimeType, parsedRuntimeEndpoint, nil
}
