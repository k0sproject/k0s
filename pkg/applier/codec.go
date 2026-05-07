/*
Copyright 2026 k0s authors

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
