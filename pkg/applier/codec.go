// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func BuildScheme(builderFuncs ...func(*runtime.Scheme) error) *runtime.Scheme {
	scheme := runtime.NewScheme()
	for _, addToScheme := range builderFuncs {
		utilruntime.Must(addToScheme(scheme))
	}
	return scheme
}

func CodecFor(scheme *runtime.Scheme) runtime.Codec {
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	return versioning.NewDefaultingCodecForScheme(scheme, serializer, serializer, nil, nil)
}
