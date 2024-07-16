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
	"github.com/k0sproject/k0s/pkg/k0scontext"
	nodeutil "k8s.io/component-helpers/node/util"
)

// GetNodename returns the node name for the node taking OS, cloud provider and override into account
func GetNodename(override string) (string, error) {
	return getNodename(context.TODO(), override)
}

// A URL that may be retrieved to determine the nodename.
type nodenameURL string

func getNodename(ctx context.Context, override string) (string, error) {
	if runtime.GOOS == "windows" {
		return getNodeNameWindows(ctx, override)
	}
	nodeName, err := nodeutil.GetHostname(override)
	if err != nil {
		return "", fmt.Errorf("failed to determine node name: %w", err)
	}
	return nodeName, nil
}

func getHostnameFromAwsMeta(ctx context.Context) string {
	const awsMetaInformationHostnameURL nodenameURL = "http://169.254.169.254/latest/meta-data/local-hostname"
	url := k0scontext.ValueOr(ctx, awsMetaInformationHostnameURL)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var s string
	err := requests.
		URL(string(url)).
		ToString(&s).
		Fetch(ctx)
	// if status code is 2XX and no transport error, we assume we are running on ec2
	if err != nil {
		return ""
	}
	return s
}

func getNodeNameWindows(ctx context.Context, override string) (string, error) {
	// if we have explicit hostnameOverride, we use it as is even on windows
	if override != "" {
		return nodeutil.GetHostname(override)
	}

	// we need to check if we have EC2 dns name available
	if h := getHostnameFromAwsMeta(ctx); h != "" {
		return h, nil
	}
	// otherwise we use the k8s hostname helper
	return nodeutil.GetHostname(override)
}
