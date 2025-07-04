/*
Copyright 2025 k0s authors

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

package runtime

import (
	"net/url"
	"path/filepath"
	"slices"

	"github.com/Microsoft/go-winio"
	"google.golang.org/grpc"
)

func newCRIRuntime(runtimeEndpoint *url.URL) *CRIRuntime {
	if runtimeEndpoint.Scheme != "npipe" {
		return &CRIRuntime{
			target:      runtimeEndpoint.String(),
			dialOptions: defaultGRPCDialOptions,
		}
	}

	return &CRIRuntime{
		target: (&url.URL{
			Scheme: "passthrough", // https://github.com/grpc/grpc-go/issues/7288#issuecomment-2141190333
			Opaque: filepath.FromSlash(runtimeEndpoint.Path),
		}).String(),

		dialOptions: slices.Concat(
			defaultGRPCDialOptions,
			[]grpc.DialOption{
				grpc.WithContextDialer(winio.DialPipeContext),
			},
		),
	}
}
