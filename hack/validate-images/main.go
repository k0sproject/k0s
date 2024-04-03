//go:build hack
// +build hack

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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"k8s.io/utils/strings/slices"

	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	var architectures []string
	var architecturesString string
	flag.StringVar(&architecturesString, "architectures", "amd64,arm64,arm", "which architectures to search for")
	flag.Parse()
	architectures = strings.Split(architecturesString, ",")
	if len(architectures) < 1 {
		panic("No architectures given")
	}
	cfg := v1beta1.DefaultClusterConfig()
	uris := airgap.GetImageURIs(cfg.Spec, false)

	var errs []error
	errs = append(errs, validateImages(uris, architectures)...)

	// Envoy doesn't have an official ARMv7 image!
	architectures = slices.Filter(nil, architectures, func(s string) bool { return s != "arm" })
	errs = append(errs, validateImages([]string{v1beta1.DefaultEnvoyProxyImage().URI()}, architectures)...)

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Not all images were valid.")
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, "Error: ", err)
		}
		os.Exit(1)
	}
}

func validateImages(uris []string, architectures []string) (errs []error) {
	for _, name := range uris {
		fmt.Println("validating image", name, "to have architectures: ", architectures)
		imageRef, err := util.ParseName(name)
		check(err)
		memoryStore := store.NewMemoryStore()
		resolver := util.NewResolver("", "", true, true)
		descriptor, err := registry.FetchDescriptor(resolver, memoryStore, imageRef)
		check(err)
		_, db, _ := memoryStore.Get(descriptor)
		switch descriptor.MediaType {
		case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
			// this is a multi-platform image descriptor; marshal to Index type
			var idx ocispec.Index
			check(json.Unmarshal(db, &idx))
			if validationErrs := validateList(name, architectures, memoryStore, descriptor, idx); validationErrs != nil {
				errs = append(errs, validationErrs...)
			}
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			errs = append(errs, fmt.Errorf("image %s has single manifest, but we need multiarch manifests", name))
		default:
			errs = append(errs, fmt.Errorf("image %s has unknown manifest type, can't validate architectures", name))
		}
	}
	return
}

func validateList(name string, architectures []string, cs *store.MemoryStore, descriptor ocispec.Descriptor, index ocispec.Index) (errs []error) {
	searchFor := map[string]bool{}

	for _, m := range index.Manifests {
		for _, platformToCheck := range architectures {
			if m.Platform.Architecture == platformToCheck {
				searchFor[platformToCheck] = true
			}
		}
	}

	for _, platform := range architectures {
		_, found := searchFor[platform]
		if !found {
			errs = append(errs, fmt.Errorf("platform %s not found for image %s", platform, name))
		}
	}

	return
}
