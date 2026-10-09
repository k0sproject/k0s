// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
