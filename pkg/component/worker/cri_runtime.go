// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

type RuntimeEndpoint = url.URL

// Parses the CRI runtime flag and returns the parsed values.
// If the flag is empty, provide k0s's defaults.
func GetContainerRuntimeEndpoint(criSocketFlag, k0sRunDir string) (*RuntimeEndpoint, error) {
	if criSocketFlag != "" {
		return parseCRISocketFlag(criSocketFlag)
	}

	return getContainerRuntimeEndpoint(k0sRunDir), nil
}

func getContainerRuntimeEndpoint(k0sRunDir string) *RuntimeEndpoint {
	if runtime.GOOS == "windows" {
		return &url.URL{Scheme: "npipe", Path: "//./pipe/containerd-containerd"}
	}

	return &url.URL{Scheme: "unix", Path: filepath.ToSlash(GetContainerdAddress(k0sRunDir))}
}

func GetContainerdAddress(k0sRunDir string) string {
	if runtime.GOOS == "windows" {
		return getContainerRuntimeEndpoint(k0sRunDir).String()
	}

	return filepath.Join(k0sRunDir, "containerd.sock")
}

func parseCRISocketFlag(criSocketFlag string) (*RuntimeEndpoint, error) {
	runtimeType, runtimeEndpoint, ok := strings.Cut(criSocketFlag, ":")
	if !ok {
		return nil, errors.New("CRI socket flag must be of the form <type>:<url>")
	}
	if runtimeType != "remote" {
		return nil, fmt.Errorf(`unknown runtime type %q, only "remote" is supported`, runtimeType)
	}
	parsedRuntimeEndpoint, err := url.Parse(runtimeEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse runtime endpoint: %w", err)
	}

	return parsedRuntimeEndpoint, nil
}
