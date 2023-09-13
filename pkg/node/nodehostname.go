/*
Copyright 2023 k0s authors

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

package node

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/carlmjohnson/requests"
	nodeutil "k8s.io/component-helpers/node/util"
)

// GetNodename returns the node name for the node taking OS, cloud provider and override into account
func GetNodename(override string) (string, error) {
	if runtime.GOOS == "windows" {
		return getNodeNameWindows(override, awsMetaInformationHostnameURL)
	}
	nodeName, err := nodeutil.GetHostname(override)
	if err != nil {
		return "", fmt.Errorf("failed to determine node name: %w", err)
	}
	return nodeName, nil
}

const awsMetaInformationHostnameURL = "http://169.254.169.254/latest/meta-data/local-hostname"

func getHostnameFromAwsMeta(url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var s string
	err := requests.
		URL(url).
		ToString(&s).
		Fetch(ctx)
	// if status code is 2XX and no transport error, we assume we are running on ec2
	if err != nil {
		return ""
	}
	return s
}

func getNodeNameWindows(override string, metadataURL string) (string, error) {
	// if we have explicit hostnameOverride, we use it as is even on windows
	if override != "" {
		return nodeutil.GetHostname(override)
	}

	// we need to check if we have EC2 dns name available
	if h := getHostnameFromAwsMeta(metadataURL); h != "" {
		return h, nil
	}
	// otherwise we use the k8s hostname helper
	return nodeutil.GetHostname(override)
}
