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

package node

import (
	"context"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/carlmjohnson/requests"
)

// A URL that may be retrieved to determine the nodename.
type nodenameURL string

func defaultNodenameOverride(ctx context.Context) string {
	// we need to check if we have EC2 dns name available
	url := k0scontext.ValueOr[nodenameURL](ctx, "http://169.254.169.254/latest/meta-data/local-hostname")

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
